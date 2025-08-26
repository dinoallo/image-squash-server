package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	imagesutil "github.com/lingdie/image-rebase-server/pkg/images"
	"github.com/lingdie/image-rebase-server/pkg/options"
	"github.com/lingdie/image-rebase-server/pkg/util"
	"github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/errdefs"
	"github.com/containerd/nerdctl/pkg/idutil/imagewalker"
	"github.com/containerd/nerdctl/pkg/imgutil"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	emptyDigest    = digest.Digest("")
	defaultAuthor  = "image-rebase-server"
	defaultMessage = "image squashed"
)

type Runtime struct {
	client    *containerd.Client
	namespace string
	logger    *logrus.Logger

	differ       containerd.DiffService
	imagestore   images.Store
	contentstore content.Store
	snapshotter  snapshots.Snapshotter
}

func (r *Runtime) initImage(ctx context.Context, opt options.Option) (*imagesutil.SquashImage, error) {
	containerImage, err := r.imagestore.Get(ctx, opt.SourceImage)
	if err != nil {
		return &imagesutil.SquashImage{}, err
	}
	targetDesc := containerImage.Target
	if !images.IsManifestType(targetDesc.MediaType) {
		return &imagesutil.SquashImage{}, fmt.Errorf("only manifest type is supported :%w", errdefs.ErrInvalidArgument)
	}

	clientImage := containerd.NewImage(r.client, containerImage)
	manifest, _, err := imgutil.ReadManifest(ctx, clientImage)
	if err != nil {
		return &imagesutil.SquashImage{}, err
	}
	config, _, err := imgutil.ReadImageConfig(ctx, clientImage)
	if err != nil {
		return &imagesutil.SquashImage{}, err
	}
	resImage := &imagesutil.SquashImage{
		ClientImage: clientImage,
		Config:      config,
		Image:       containerImage,
		Manifest:    manifest,
	}
	return resImage, err
}

func (r *Runtime) generateSquashLayer(opt options.Option, image *imagesutil.SquashImage) ([]ocispec.Descriptor, error) {
	// get the layer descriptors by the layer digest
	if opt.SquashLayerDigest != "" {
		find := false
		var res []ocispec.Descriptor
		for _, layer := range image.Manifest.Layers {
			if layer.Digest.String() == opt.SquashLayerDigest {
				find = true
			}
			if find {
				res = append(res, layer)
			}
		}
		if !find {
			return nil, fmt.Errorf("layer not found")
		}
		return res, nil
	}

	// get the layer descriptors by the layer count
	if opt.SquashLayerCount > 1 && opt.SquashLayerCount <= len(image.Manifest.Layers) {
		return image.Manifest.Layers[len(image.Manifest.Layers)-opt.SquashLayerCount:], nil
	}
	return nil, fmt.Errorf("invalid squash option: %w", errdefs.ErrInvalidArgument)
}

func (r *Runtime) applyLayersToSnapshot(ctx context.Context, mount []mount.Mount, layers []ocispec.Descriptor) error {
	for _, layer := range layers {
		r.logger.Infof("apply layer %s to snapshot", layer.Digest)
		start := time.Now()
		if _, err := r.differ.Apply(ctx, layer, mount); err != nil {
			return err
		}
		elapsed := time.Since(start)
		r.logger.Infof("apply layer %s to snapshot cost %s", layer.Digest, elapsed)
	}
	return nil
}

// createDiff creates a diff from the snapshot
func (r *Runtime) createDiff(ctx context.Context, snapshotName string) (ocispec.Descriptor, digest.Digest, error) {
	r.logger.Infof("create diff for snapshot %s", snapshotName)
	start := time.Now()

	// // Create a zstd compressor with high compression level
	// zstdCompressor := func(dest io.Writer, mediaType string) (io.WriteCloser, error) {
	// 	// Use zstd with high compression level for better compression ratio
	// 	// while maintaining good speed
	// 	encoder, err := zstd.NewWriter(dest, zstd.WithEncoderLevel(zstd.SpeedFastest))
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	return encoder, nil
	// }

	// Create diff with custom compressor
	// newDesc, err := rootfs.CreateDiff(ctx, snapshotName, r.snapshotter, r.differ, diff.WithCompressor(zstdCompressor))
	newDesc, err := rootfs.CreateDiff(ctx, snapshotName, r.snapshotter, r.differ)
	if err != nil {
		return ocispec.Descriptor{}, "", err
	}
	info, err := r.contentstore.Info(ctx, newDesc.Digest)
	if err != nil {
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	diffIDStr, ok := info.Labels["containerd.io/uncompressed"]
	if !ok {
		return ocispec.Descriptor{}, digest.Digest(""), fmt.Errorf("invalid differ response with no diffID")
	}
	diffID, err := digest.Parse(diffIDStr)
	if err != nil {
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	elapsed := time.Since(start)
	r.logger.Infof("create diff for snapshot %s cost %s", snapshotName, elapsed)
	return ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2LayerGzip,
		Digest:    newDesc.Digest,
		Size:      info.Size,
	}, diffID, nil
}

func (r *Runtime) generateBaseImageConfig(ctx context.Context, image *imagesutil.SquashImage, remainingLayerCount int) (ocispec.Image, error) {
	// generate squash image config
	orginalConfig, _, err := imgutil.ReadImageConfig(ctx, image.ClientImage) // aware of img.platform
	if err != nil {
		return ocispec.Image{}, err
	}

	var history []ocispec.History
	var count int
	for _, h := range orginalConfig.History {
		// if empty layer, add to history, be careful with the last layer that is empty
		if h.EmptyLayer {
			history = append(history, h)
			continue
		}
		// if not empty layer, add to history, check if count+1 <= remainingLayerCount to see if we need to add more
		if count+1 <= remainingLayerCount {
			history = append(history, h)
			count++
		} else {
			break
		}
	}
	cTime := time.Now()
	return ocispec.Image{
		Created:  &cTime,
		Author:   orginalConfig.Author,
		Platform: orginalConfig.Platform,
		Config:   orginalConfig.Config,
		RootFS: ocispec.RootFS{
			Type:    orginalConfig.RootFS.Type,
			DiffIDs: orginalConfig.RootFS.DiffIDs[:remainingLayerCount],
		},
		History: history,
	}, nil
}

func (r *Runtime) writeContentsForImage(ctx context.Context, snName string, newConfig ocispec.Image, baseImageLayers []ocispec.Descriptor, diffLayerDesc ocispec.Descriptor) (ocispec.Descriptor, digest.Digest, error) {
	// write image contents to content store
	newConfigJSON, err := json.Marshal(newConfig)
	if err != nil {
		return ocispec.Descriptor{}, emptyDigest, err
	}

	configDesc := ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2Config,
		Digest:    digest.FromBytes(newConfigJSON),
		Size:      int64(len(newConfigJSON)),
	}

	layers := append(baseImageLayers, diffLayerDesc)

	newMfst := struct {
		MediaType string `json:"mediaType,omitempty"`
		ocispec.Manifest
	}{
		MediaType: images.MediaTypeDockerSchema2Manifest,
		Manifest: ocispec.Manifest{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			Config: configDesc,
			Layers: layers,
		},
	}

	newMfstJSON, err := json.MarshalIndent(newMfst, "", "    ")
	if err != nil {
		return ocispec.Descriptor{}, emptyDigest, err
	}

	newMfstDesc := ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2Manifest,
		Digest:    digest.FromBytes(newMfstJSON),
		Size:      int64(len(newMfstJSON)),
	}

	// new manifest should reference the layers and config content
	labels := map[string]string{
		"containerd.io/gc.ref.content.0": configDesc.Digest.String(),
	}
	for i, l := range layers {
		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i+1)] = l.Digest.String()
	}

	err = content.WriteBlob(ctx, r.contentstore, newMfstDesc.Digest.String(), bytes.NewReader(newMfstJSON), newMfstDesc, content.WithLabels(labels))
	if err != nil {
		return ocispec.Descriptor{}, emptyDigest, err
	}

	// config should reference to snapshotter
	labelOpt := content.WithLabels(map[string]string{
		fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", snName): identity.ChainID(newConfig.RootFS.DiffIDs).String(),
	})
	err = content.WriteBlob(ctx, r.contentstore, configDesc.Digest.String(), bytes.NewReader(newConfigJSON), configDesc, labelOpt)
	if err != nil {
		return ocispec.Descriptor{}, emptyDigest, err
	}
	return newMfstDesc, configDesc.Digest, nil
}

func (r *Runtime) createSquashImage(ctx context.Context, img images.Image) (images.Image, error) {
	newImg, err := r.imagestore.Update(ctx, img)
	if err != nil {
		// if err has `not found` in the message then create the image, otherwise return the error
		if !errdefs.IsNotFound(err) {
			return newImg, fmt.Errorf("failed to update new squash image %s: %w", img.Name, err)
		}
		if _, err := r.imagestore.Create(ctx, img); err != nil {
			return newImg, fmt.Errorf("failed to create new squash image %s: %w", img.Name, err)
		}
	}
	return newImg, nil
}

// generateCommitImageConfig returns commit oci image config based on the container's image.
func (r *Runtime) generateCommitImageConfig(ctx context.Context, baseImg images.Image, baseConfig ocispec.Image, diffID digest.Digest) (ocispec.Image, error) {
	createdTime := time.Now()
	arch := baseConfig.Architecture
	if arch == "" {
		arch = runtime.GOARCH
		r.logger.Warnf("assuming arch=%q", arch)
	}
	os := baseConfig.OS
	if os == "" {
		os = runtime.GOOS
		r.logger.Warnf("assuming os=%q", os)
	}
	author := strings.TrimSpace(defaultAuthor) //TODO: make this configurable
	if author == "" {
		author = baseConfig.Author
	}
	comment := strings.TrimSpace(defaultMessage) //TODO: make this configurable

	baseImageDigest := strings.Split(baseImg.Target.Digest.String(), ":")[1][:12]
	return ocispec.Image{
		Platform: ocispec.Platform{
			Architecture: arch,
			OS:           os,
		},

		Created: &createdTime,
		Author:  author,
		Config:  baseConfig.Config,
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: append(baseConfig.RootFS.DiffIDs, diffID),
		},
		History: append(baseConfig.History, ocispec.History{
			Created:    &createdTime,
			CreatedBy:  fmt.Sprintf("squash from %s", baseImageDigest),
			Author:     author,
			Comment:    comment,
			EmptyLayer: false,
		}),
	}, nil
}

func NewRuntime(client *containerd.Client, namespace string) (*Runtime, error) {
	return &Runtime{
		client:       client,
		namespace:    namespace,
		differ:       client.DiffService(),
		imagestore:   client.ImageService(),
		contentstore: client.ContentStore(),
		logger:       logrus.New(),
		// use default snapshotter
		snapshotter: client.SnapshotService(""),
	}, nil
}

func (r *Runtime) Squash(ctx context.Context, opt options.Option) error {
	var srcName string
	walker := &imagewalker.ImageWalker{
		Client: r.client,
		OnFound: func(ctx context.Context, found imagewalker.Found) error {
			if srcName == "" {
				srcName = found.Image.Name
			}
			return nil
		},
	}
	matchCount, err := walker.Walk(ctx, opt.SourceImage)
	if err != nil {
		return err
	}
	if matchCount < 1 {
		return fmt.Errorf("source image %q not found", opt.SourceImage)
	} else if matchCount > 1 {
		return fmt.Errorf("multiple source images found for %q", opt.SourceImage)
	}
	opt.SourceImage = srcName
	ctx = namespaces.WithNamespace(ctx, r.namespace)
	// init image
	image, err := r.initImage(ctx, opt)
	if err != nil {
		return err
	}
	// generate squash layers
	sLayers, err := r.generateSquashLayer(opt, image)
	if err != nil {
		return err
	}
	remainingLayerCount := len(image.Manifest.Layers) - len(sLayers)
	// Don't gc me and clean the dirty data after 1 hour! //TODO: check me
	/*
		ctx, done, err := r.client.WithLease(ctx, leases.WithRandomID(), leases.WithExpiration(24*time.Hour))
		if err != nil {
			return err
		}
		defer done(ctx)
	*/
	start := time.Now()
	elapsed := time.Since(start)
	baseImage, err := r.generateBaseImageConfig(ctx, image, remainingLayerCount)
	if err != nil {
		return err
	}
	start = time.Now()
	diffLayerDesc, diffID, _, err := r.applyDiffLayer(ctx, baseImage, r.snapshotter, sLayers)
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	r.logger.Infof("apply diff layer cost %s", elapsed)
	imageConfig, err := r.generateCommitImageConfig(ctx, image.Image, baseImage, diffID)
	if err != nil {
		return fmt.Errorf("failed to generate commit image config: %w", err)
	}
	start = time.Now()
	commitManifestDesc, _, err := r.writeContentsForImage(ctx, "overlayfs", imageConfig, image.Manifest.Layers[:remainingLayerCount], diffLayerDesc)
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	r.logger.Infof("write the content for image cost %s", elapsed)
	nImg := images.Image{
		Name:      opt.TargetImage,
		Target:    commitManifestDesc,
		UpdatedAt: time.Now(),
	}
	elapsed = time.Since(start)
	r.logger.Infof("generate squash image cost %s", elapsed)
	// create squash image
	start = time.Now()
	_, err = r.createSquashImage(ctx, nImg)
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	r.logger.Infof("create squash image cost %s", elapsed)
	cimg := containerd.NewImage(r.client, nImg)
	if err := cimg.Unpack(ctx, "overlayfs"); err != nil {
		return err
	}

	return nil
}
func (r *Runtime) applyDiffLayer(ctx context.Context, baseImg ocispec.Image, sn snapshots.Snapshotter, layers []ocispec.Descriptor) (
	diffLayerDesc ocispec.Descriptor, diffID digest.Digest, snapshotID string, retErr error) {
	var (
		key    = util.UniquePart()
		parent = identity.ChainID(baseImg.RootFS.DiffIDs).String()
	)

	m, err := sn.Prepare(ctx, key, parent)
	if err != nil {
		return diffLayerDesc, diffID, snapshotID, err
	}

	defer func() {
		if retErr != nil {
			// NOTE: the snapshotter should be hold by lease. Even
			// if the cleanup fails, the containerd gc can delete it.
			if err := sn.Remove(ctx, key); err != nil {
				r.logger.Warnf("failed to cleanup aborted apply %s: %v", key, err)
			}
		}
	}()

	err = r.applyLayersToSnapshot(ctx, m, layers)
	if err != nil {
		r.logger.Warnf("failed to apply layers to snapshot %s: %v", key, err)
		return diffLayerDesc, diffID, snapshotID, err
	}
	diffLayerDesc, diffID, err = r.createDiff(ctx, key)
	if err != nil {
		return diffLayerDesc, diffID, snapshotID, fmt.Errorf("failed to export layer: %w", err)
	}

	// commit snapshot
	snapshotID = identity.ChainID(append(baseImg.RootFS.DiffIDs, diffID)).String()

	if err = sn.Commit(ctx, snapshotID, key); err != nil {
		if errdefs.IsAlreadyExists(err) {
			return diffLayerDesc, diffID, snapshotID, nil
		}
		return diffLayerDesc, diffID, snapshotID, err
	}
	return diffLayerDesc, diffID, snapshotID, nil
}
