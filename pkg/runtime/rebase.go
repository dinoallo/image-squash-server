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
	rebaseStart := time.Now()
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
	var rebaseToDoList []string
	if opt.AutoSquash {
		rebaseToDoList = getSquashAll(layersToRebase)
	} else {
		rebaseToDoList = getAllPick(layersToRebase)
	}
	// diffIDs for the layers we are going to rebase (needed so we can reuse a single layer without squashing)
	rebaseDiffIDs := origImage.Config.RootFS.DiffIDs[len(baseImage.Manifest.Layers):]
	// Don't gc me and clean the dirty data after 1 hour! (or the temp snapshot may be gced when we are debugging)
	ctx, done, err := r.client.WithLease(ctx, leases.WithRandomID(), leases.WithExpiration(24*time.Hour))
	if err != nil {
		return err
	}
	defer done(ctx)
	modifyLayerStart := time.Now()
	newLayers, newDiffIDs, err := r.modifyLayers(ctx, newBaseImage.Config, layersToRebase, rebaseDiffIDs, rebaseToDoList)
	if err != nil {
		r.logger.Errorf("failed to modify layers: %v", err)
		return err
	}
	modifyLayerElapsed := time.Since(modifyLayerStart)
	r.logger.Infof("modify layers took %s", modifyLayerElapsed)
	// generate new image config
	newBaseImageConfig := newBaseImage.Config
	newImageConfig, err := r.GenerateImageConfig(ctx, newBaseImage.Image, newBaseImageConfig, newDiffIDs)
	if err != nil {
		r.logger.Errorf("failed to generate new image config: %v", err)
		return err
	}
	writeContentsForImageStart := time.Now()
	commitManifestDesc, _, err := r.WriteContentsForImage(ctx, "overlayfs", newImageConfig, newBaseImage.Manifest.Layers, newLayers)
	if err != nil {
		r.logger.Errorf("failed to write contents for image %q: %v", opt.NewImage, err)
		return err
	}
	writeContentsForImageElapsed := time.Since(writeContentsForImageStart)
	r.logger.Infof("write contents for image took %s", writeContentsForImageElapsed)
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
	rebaseElapsed := time.Since(rebaseStart)
	r.logger.Infof("rebase image %q to new base image %q successfully, new image: %q, cost %s", opt.OriginalImage, opt.NewBaseImage, opt.NewImage, rebaseElapsed)
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

func getAllPick(layers []ocispec.Descriptor) []string {
	toDoList := make([]string, len(layers))
	for i := range layers {
		toDoList[i] = "pick"
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

func (r *Runtime) modifyLayers(ctx context.Context, baseImg ocispec.Image, layersToRebase []ocispec.Descriptor, rebaseDiffIDs []digest.Digest, rebaseToDoList []string) ([]ocispec.Descriptor, []digest.Digest, error) {
	var (
		layersToSquash = []ocispec.Descriptor{}
		newLayers      = []ocispec.Descriptor{}
		newDiffIDs     = []digest.Digest{}
		parentDiffIDs  = baseImg.RootFS.DiffIDs
	)
	if len(layersToRebase) != len(rebaseDiffIDs) {
		return nil, nil, fmt.Errorf("layersToRebase and rebaseDiffIDs length mismatch: %d != %d", len(layersToRebase), len(rebaseDiffIDs))
	}
	groupStartIdx := -1 // index in layersToRebase / rebaseDiffIDs where current group started

	flushGroup := func() error {
		if len(layersToSquash) == 0 {
			return nil
		}
		if len(layersToSquash) == 1 {
			// Reuse the original single layer & its diffID instead of re-squashing
			origIdx := groupStartIdx
			layer := layersToSquash[0]
			diffID := rebaseDiffIDs[origIdx]
			newLayers = append(newLayers, layer)
			newDiffIDs = append(newDiffIDs, diffID)
			parentDiffIDs = append(parentDiffIDs, diffID)
			layersToSquash = layersToSquash[:0]
			return nil
		}
		layer, diffID, err := r.squashLayers(ctx, layersToSquash, parentDiffIDs)
		if err != nil {
			return fmt.Errorf("failed to squash layers: %w", err)
		}
		newLayers = append(newLayers, layer)
		newDiffIDs = append(newDiffIDs, diffID)
		parentDiffIDs = append(parentDiffIDs, diffID)
		layersToSquash = layersToSquash[:0]
		return nil
	}
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
				if err := flushGroup(); err != nil {
					return newLayers, nil, err
				}
			}
			layersToSquash = []ocispec.Descriptor{layer}
			groupStartIdx = i

		default:
			return nil, nil, fmt.Errorf("unknown action %q", action)
		}
		if groupStartIdx == -1 { // first time we add a layer (could be i==0 pick)
			groupStartIdx = i
		}
	}
	// remember to handle the leftover items in layersToSquash
	if len(layersToSquash) > 0 {
		if err := flushGroup(); err != nil {
			return newLayers, nil, err
		}
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
