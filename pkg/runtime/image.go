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
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/nerdctl/pkg/idutil/imagewalker"
	"github.com/containerd/nerdctl/pkg/imgutil"
	imagesutil "github.com/lingdie/image-manip-server/pkg/images"
	"github.com/opencontainers/go-digest"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultAuthor  = "image-manip"
	defaultMessage = "image rebased"
)

func (r *Runtime) FindImage(ctx context.Context, imageRef string) (string, error) {
	var srcName string
	ctx = namespaces.WithNamespace(ctx, r.namespace)
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

func (r *Runtime) GetImage(ctx context.Context, imageRef string) (*imagesutil.Image, error) {
	image, err := r.FindImage(ctx, imageRef)
	if err != nil {
		return &imagesutil.Image{}, err
	}
	containerImage, err := r.imagestore.Get(ctx, image)
	if err != nil {
		return &imagesutil.Image{}, err
	}
	targetDesc := containerImage.Target
	if !images.IsManifestType(targetDesc.MediaType) {
		return &imagesutil.Image{}, fmt.Errorf("only manifest type is supported :%w", errdefs.ErrInvalidArgument)
	}

	clientImage := containerd.NewImage(r.client, containerImage)
	manifest, _, err := imgutil.ReadManifest(ctx, clientImage)
	if err != nil {
		return &imagesutil.Image{}, err
	}
	config, _, err := imgutil.ReadImageConfig(ctx, clientImage)
	if err != nil {
		return &imagesutil.Image{}, err
	}
	resImage := &imagesutil.Image{
		ClientImage: clientImage,
		Config:      config,
		Image:       containerImage,
		Manifest:    manifest,
	}
	return resImage, err
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

// GenerateImageConfig returns oci image config based on the base image and the rebased layers
func (r *Runtime) GenerateImageConfig(ctx context.Context, baseImg images.Image, baseConfig ocispec.Image, newDiffIDs []digest.Digest) (ocispec.Image, error) {
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
			DiffIDs: append(baseConfig.RootFS.DiffIDs, newDiffIDs...),
		},
		History: append(baseConfig.History, ocispec.History{
			Created:    &createdTime,
			CreatedBy:  defaultAuthor + " (rebased onto " + baseImageDigest + ")",
			Author:     author,
			Comment:    comment,
			EmptyLayer: false,
		}),
	}, nil
}
