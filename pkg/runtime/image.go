package runtime

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/nerdctl/pkg/idutil/imagewalker"
	"github.com/containerd/nerdctl/pkg/imgutil"
	imagesutil "github.com/lingdie/image-manip-server/pkg/images"
	"github.com/opencontainers/go-digest"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultAuthor  = "image-manip"
	defaultMessage = "layer merged by image-manip"
)

func (r *Runtime) FindImage(ctx context.Context, imageRef string) (string, error) {
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
	matchCount, err := walker.Walk(ctx, imageRef)
	if err != nil {
		return "", err
	}
	if matchCount < 1 {
		return "", fmt.Errorf("image %q not found", imageRef)
	} else if matchCount > 1 {
		return "", fmt.Errorf("multiple images found for %q", imageRef)
	}
	return srcName, nil
}

func (r *Runtime) GetImage(ctx context.Context, imageRef string) (imagesutil.Image, error) {
	imageName, err := r.FindImage(ctx, imageRef)
	image := imagesutil.Image{}
	if err != nil {
		return image, err
	}
	containerImage, err := r.imagestore.Get(ctx, imageName)
	if err != nil {
		return image, err
	}
	//TODO: is this check required?
	/*
		targetDesc := containerImage.Target
			if !images.IsManifestType(targetDesc.MediaType) {
				return &imagesutil.Image{}, fmt.Errorf("only manifest type is supported :%w", errdefs.ErrInvalidArgument)
			}*/

	clientImage := containerd.NewImage(r.client, containerImage)
	manifest, _, err := imgutil.ReadManifest(ctx, clientImage)
	if err != nil {
		return imagesutil.Image{}, err
	}
	config, _, err := imgutil.ReadImageConfig(ctx, clientImage)
	if err != nil {
		return imagesutil.Image{}, err
	}
	image = imagesutil.Image{
		ClientImage: clientImage,
		Config:      config,
		Image:       containerImage,
		Manifest:    manifest,
	}
	return image, err
}

func (r *Runtime) UpdateImage(ctx context.Context, img images.Image) (images.Image, error) {
	newImg, err := r.imagestore.Update(ctx, img)
	if err != nil {
		// if err has `not found` in the message then create the image, otherwise return the error
		if !errdefs.IsNotFound(err) {
			return newImg, fmt.Errorf("failed to update new image %s: %w", img.Name, err)
		}
		if _, err := r.imagestore.Create(ctx, img); err != nil {
			return newImg, fmt.Errorf("failed to create new image %s: %w", img.Name, err)
		}
	}
	return newImg, nil
}

func (r *Runtime) WriteBack(ctx context.Context, baseConfig ocispec.Image, baseLayers []ocispec.Descriptor, newLayers Layers) (ocispec.Descriptor, error) {
	// generate image config
	imageConfig, err := r.GenerateMergedImageConfig(ctx, baseConfig, newLayers)
	if err != nil {
		r.logger.Errorf("failed to generate new image config: %v", err)
		return ocispec.Descriptor{}, err
	}
	allLayers := NewLayers(
		baseLayers,
		baseConfig.RootFS.DiffIDs,
	)
	allLayers.AppendLayers(newLayers)
	// write image metadata
	writeContentsForImageStart := time.Now()
	manifestDesc, err := r.WriteImageMetadata(ctx, imageConfig, allLayers.Descriptors)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	writeContentsForImageElapsed := time.Since(writeContentsForImageStart)
	r.logger.Infof("write contents for image took %s", writeContentsForImageElapsed)
	return manifestDesc, nil
}

func (r *Runtime) UnpackImage(ctx context.Context, imageName string, manifestDesc ocispec.Descriptor) error {
	nImg := images.Image{
		Name:      imageName,
		Target:    manifestDesc,
		UpdatedAt: time.Now(),
	}
	_, err := r.UpdateImage(ctx, nImg)
	if err != nil {
		r.logger.Errorf("failed to update image %q: %v", imageName, err)
		return err
	}
	cimg := containerd.NewImage(r.client, nImg)
	// unpack image to the snapshot storage
	if err := cimg.Unpack(ctx, r.snapshotterName); err != nil {
		r.logger.Errorf("failed to unpack image %q: %v", imageName, err)
		return err
	}
	return nil
}

// GenerateMergedImageConfig generates a new image config by merging the base image config and the new layers.
func (r *Runtime) GenerateMergedImageConfig(ctx context.Context, baseConfig ocispec.Image, newLayers Layers) (ocispec.Image, error) {
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
	return ocispec.Image{
		Platform: ocispec.Platform{
			Architecture: arch,
			OS:           os,
		},
		Created: &createdTime,
		Author:  author,
		Config:  baseConfig.Config,
		RootFS:  generateRootFS(baseConfig.RootFS, newLayers),
		History: generateHistory(baseConfig.History, newLayers),
	}, nil
}

func generateRootFS(baseRootfs ocispec.RootFS, layers Layers) ocispec.RootFS {
	diffIDs := make([]digest.Digest, 0, len(baseRootfs.DiffIDs)+len(layers.DiffIDs))
	copy(diffIDs, baseRootfs.DiffIDs)
	return ocispec.RootFS{
		Type:    "layers",
		DiffIDs: append(diffIDs, layers.DiffIDs...),
	}
}

func generateHistory(_history []ocispec.History, layers Layers) []ocispec.History {
	history := make([]ocispec.History, 0, len(_history)+len(layers.Descriptors))
	copy(history, _history)
	author := strings.TrimSpace(defaultAuthor)   //TODO: make this configurable
	comment := strings.TrimSpace(defaultMessage) //TODO: make this configurable
	for _, layer := range layers.Descriptors {
		history = append(history, ocispec.History{
			Created:    nil,
			CreatedBy:  "ADD " + layer.Digest.String() + " in " + layer.MediaType,
			Author:     author,
			Comment:    comment,
			EmptyLayer: false,
		})
	}
	return history
}
