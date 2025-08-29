package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	imagesutil "github.com/lingdie/image-manip-server/pkg/images"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (r *Runtime) Rebase(ctx context.Context, opt options.RebaseOption) error {
	//TODO: implement me
	// get the original image
	r.logger.Infof("start to rebase image %q to new base image %q", opt.OriginalImage, opt.NewBaseImage)
	origImage, err := r.GetImage(ctx, opt.OriginalImage)
	if err != nil {
		r.logger.Errorf("failed to get original image %q: %v", opt.OriginalImage, err)
		return err
	}
	// get the base image
	baseImage, err := r.GetImage(ctx, opt.BaseImage)
	if err != nil {
		r.logger.Errorf("failed to get base image %q: %v", opt.BaseImage, err)
		return err
	}
	layersToRebase, err := r.generateLayersToRebase(origImage, baseImage)
	if err != nil {
		r.logger.Errorf("failed to generate layers to rebase: %v", err)
		return err
	}
	// get the new base image
	newBaseImage, err := r.GetImage(ctx, opt.NewBaseImage)
	if err != nil {
		r.logger.Errorf("failed to get new base image %q: %v", opt.NewBaseImage, err)
		return err
	}
	rebaseToDoList := getSquashAll(layersToRebase)
	// Don't gc me and clean the dirty data after 1 hour! (or the temp snapshot may be gced when we are debugging)
	ctx, done, err := r.client.WithLease(ctx, leases.WithRandomID(), leases.WithExpiration(60*time.Minute))
	if err != nil {
		return err
	}
	defer done(ctx)
	newLayers, newDiffIDs, err := r.modifyLayers(ctx, newBaseImage.Config, layersToRebase, rebaseToDoList)
	if err != nil {
		r.logger.Errorf("failed to modify layers: %v", err)
		return err
	}
	newBaseImageConfig := newBaseImage.Config
	newImageConfig, err := r.GenerateImageConfig(ctx, newBaseImage.Image, newBaseImageConfig, newDiffIDs)
	if err != nil {
		r.logger.Errorf("failed to generate new image config: %v", err)
		return err
	}
	commitManifestDesc, _, err := r.WriteContentsForImage(ctx, "overlayfs", newImageConfig, newBaseImage.Manifest.Layers, newLayers)
	if err != nil {
		r.logger.Errorf("failed to write contents for image %q: %v", opt.NewImage, err)
		return err
	}
	nImg := images.Image{
		Name:      opt.NewImage,
		Target:    commitManifestDesc,
		UpdatedAt: time.Now(),
	}
	_, err = r.UpdateImage(ctx, nImg)
	if err != nil {
		r.logger.Errorf("failed to update image %q: %v", opt.NewImage, err)
		return err
	}
	cimg := containerd.NewImage(r.client, nImg)
	// unpack image to the snapshot storage
	if err := cimg.Unpack(ctx, "overlayfs"); err != nil {
		r.logger.Errorf("failed to unpack image %q: %v", opt.NewImage, err)
		return err
	}

	return nil
}

func getSquashAll(layers []ocispec.Descriptor) []string {
	var toDoList []string
	for i := range layers {
		if i == 0 {
			toDoList = append(toDoList, "pick")
		} else {
			toDoList = append(toDoList, "fixup")
		}
	}
	return toDoList
}

func (r *Runtime) generateLayersToRebase(origImage, baseImage *imagesutil.Image) ([]ocispec.Descriptor, error) {
	oldBaseLayers := baseImage.Manifest.Layers
	origLayers := origImage.Manifest.Layers
	if len(oldBaseLayers) > len(origLayers) {
		return nil, fmt.Errorf("image %q is not based on %q (too few layers)", origImage.Image.Name, baseImage.Image.Name)
	}
	//TODO: optimize this
	for i, l := range oldBaseLayers {
		oldLayerDigest := l.Digest
		origLayerDigest := origLayers[i].Digest
		if oldLayerDigest != origLayerDigest {
			return nil, fmt.Errorf("image %q is not based on %q (layer %q != layer %q)", origImage.Image.Name, baseImage.Image.Name, oldLayerDigest, origLayerDigest)
		}
	}
	return origImage.Manifest.Layers[len(oldBaseLayers):len(origLayers)], nil
}

func (r *Runtime) modifyLayers(ctx context.Context, baseImg ocispec.Image, layersToRebase []ocispec.Descriptor, rebaseToDoList []string) ([]ocispec.Descriptor, []digest.Digest, error) {
	var (
		layersToSquash = []ocispec.Descriptor{}
		newLayers      = []ocispec.Descriptor{}
		newDiffIDs     = []digest.Digest{}
		parentDiffIDs  = baseImg.RootFS.DiffIDs
	)
	for i, layer := range layersToRebase {
		action := rebaseToDoList[i]
		switch action {
		case "fixup":
			if i == 0 {
				// the first layer cannot be fixed up
				return newLayers, nil, fmt.Errorf("the first layer cannot be fixed up")
			}
			layersToSquash = append(layersToSquash, layer)
		case "pick":
			if len(layersToSquash) > 0 {
				layer, diffID, err := r.squashLayers(ctx, layersToSquash, parentDiffIDs)
				if err != nil {
					return newLayers, nil, fmt.Errorf("failed to squash layers: %w", err)
				}
				newLayers = append(newLayers, layer)
				newDiffIDs = append(newDiffIDs, diffID)
				parentDiffIDs = append(parentDiffIDs, diffID)
			}
			layersToSquash = []ocispec.Descriptor{layer}

		default:
			return nil, nil, fmt.Errorf("unknown action %q", action)
		}
	}
	// remember to handle the leftover items in layersToSquash
	if len(layersToSquash) > 0 {
		layer, diffID, err := r.squashLayers(ctx, layersToSquash, parentDiffIDs)
		if err != nil {
			return newLayers, nil, fmt.Errorf("failed to squash layers: %w", err)
		}
		newLayers = append(newLayers, layer)
		newDiffIDs = append(newDiffIDs, diffID)
	}
	return newLayers, newDiffIDs, nil
}

func (r *Runtime) squashLayers(ctx context.Context, layersToSquash []ocispec.Descriptor, parentDiffIDs []digest.Digest) (ocispec.Descriptor, digest.Digest, error) {
	newLayer, diffID, _, err := r.createSnapshot(ctx, parentDiffIDs, r.snapshotter, layersToSquash)
	if err != nil {
		return ocispec.Descriptor{}, digest.Digest(""), fmt.Errorf("failed to apply layers to snapshot: %w", err)
	}
	return newLayer, diffID, nil
}
