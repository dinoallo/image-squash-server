package runtime

import (
	"context"
	"fmt"
	"time"

	imageutil "github.com/lingdie/image-manip-server/pkg/images"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (r *Runtime) Rebase(ctx context.Context, opt options.RebaseOptions) error {
	// get the original image
	r.logger.Infof("start to rebase image %q to layer diff ID %q", opt.OriginalImage, opt.BaseLayer)
	rebaseStart := time.Now()
	var (
		origImage  imageutil.Image
		baseConfig ocispec.Image
		err        error
	)
	origImage, err = r.GetImage(ctx, opt.OriginalImage)
	if err != nil {
		r.logger.Errorf("failed to get original image %q: %v", opt.OriginalImage, err)
		return err
	}
	if opt.OriginalImage != opt.NewBaseImage {
		newBaseImage, err := r.GetImage(ctx, opt.NewBaseImage)
		if err != nil {
			r.logger.Errorf("failed to get new base image %q: %v", opt.NewBaseImage, err)
			return err
		}
		baseConfig = newBaseImage.Config
	} else {
		baseConfig = origImage.Config
	}

	baseLayerDigest, err := digest.Parse(opt.BaseLayer)
	if err != nil {
		r.logger.Errorf("failed to parse base layer ref %q: %v", opt.BaseLayer, err)
		return err
	}
	layersToRebase, baseLayerIndex, err := r.generateLayersToRebase(origImage, baseLayerDigest)
	if err != nil {
		r.logger.Errorf("failed to generate layers to rebase: %v", err)
		return err
	}
	root := NewSnapshot(origImage.Config.RootFS.DiffIDs[:baseLayerIndex])
	var rebaseToDoList []string
	if opt.AutoSquash {
		rebaseToDoList = getSquashAll(layersToRebase)
	} else {
		rebaseToDoList = getAllPick(layersToRebase)
	}
	modifyLayerStart := time.Now()
	newLayers, err := r.modifyLayers(ctx, root, layersToRebase, rebaseToDoList)
	if err != nil {
		r.logger.Errorf("failed to modify layers: %v", err)
		return err
	}
	modifyLayerElapsed := time.Since(modifyLayerStart)
	r.logger.Infof("modify layers took %s", modifyLayerElapsed)
	baseLayers := origImage.Manifest.Layers[:baseLayerIndex]
	// write back the new image
	if manifestDesc, err := r.WriteBack(ctx, baseConfig, baseLayers, newLayers); err != nil {
		return err
	} else if err := r.UnpackImage(ctx, opt.NewImage, manifestDesc); err != nil {
		r.logger.Errorf("failed to unpack image %q: %v", opt.NewImage, err)
		return err
	}
	rebaseElapsed := time.Since(rebaseStart)
	r.logger.Infof("rebase image %q successfully, new image: %q, cost %s", opt.OriginalImage, opt.NewImage, rebaseElapsed)
	return nil
}

func getSquashAll(layers []Layer) []string {
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

func getAllPick(layers []Layer) []string {
	toDoList := make([]string, len(layers))
	for i := range layers {
		toDoList[i] = "pick"
	}
	return toDoList
}

func (r *Runtime) generateLayersToRebase(origImage imageutil.Image, targetLayerRef digest.Digest) ([]Layer, int, error) {
	if len(origImage.Manifest.Layers) != len(origImage.Config.RootFS.DiffIDs) {
		return nil, -1, fmt.Errorf("invalid image %q: number of layers in manifest (%d) does not match number of diff IDs in config (%d)", origImage.Image.Name, len(origImage.Manifest.Layers), len(origImage.Config.RootFS.DiffIDs))
	}
	// find the target layer index
	imageLayers := make([]Layer, len(origImage.Manifest.Layers))
	targetLayerIdx := -1
	//TODO: optimize this
	for i, l := range origImage.Manifest.Layers {
		if l.Digest == targetLayerRef {
			targetLayerIdx = i
			break
		}
		imageLayers[i] = Layer{
			Desc:   l,
			DiffID: origImage.Config.RootFS.DiffIDs[i],
		}
	}
	if targetLayerIdx == -1 {
		return nil, -1, fmt.Errorf("target layer %q not found in original image %q", targetLayerRef, origImage.Image.Name)
	}

	return imageLayers[targetLayerIdx:], targetLayerIdx, nil
}

func (r *Runtime) modifyLayers(ctx context.Context, root Snapshot, layersToRebase []Layer, rebaseToDoList []string) (Layers, error) {
	var (
		layersToSquash          = []Layer{}
		newLayers               = NewEmptyLayers()
		currentParent  Snapshot = root.Clone()
	)
	groupStartIdx := -1 // index in layersToRebase / rebaseDiffIDs where current group started

	update := func(layer Layer) {
		newLayers.AppendLayer(layer)
		currentParent = currentParent.NewChild(layer.DiffID)
		layersToSquash = layersToSquash[:0]
	}

	flushGroup := func() error {
		if len(layersToSquash) == 0 {
			return nil
		}
		if len(layersToSquash) == 1 {
			// Reuse the original single layer & its diffID instead of re-squashing
			layer := layersToSquash[0]
			update(layer)
			return nil
		}
		layer, err := r.squashLayers(ctx, currentParent, layersToSquash)
		if err != nil {
			return fmt.Errorf("failed to squash layers: %w", err)
		}
		update(layer)
		return nil
	}
	for i, layer := range layersToRebase {
		action := rebaseToDoList[i]
		switch action {
		case "fixup":
			if i == 0 {
				// the first layer cannot be fixed up
				return newLayers, fmt.Errorf("the first layer cannot be fixed up")
			}
			layersToSquash = append(layersToSquash, layer)
		case "pick":
			if len(layersToSquash) > 0 {
				if err := flushGroup(); err != nil {
					return newLayers, err
				}
			}
			layersToSquash = []Layer{layer}
			groupStartIdx = i

		default:
			return newLayers, fmt.Errorf("unknown action %q", action)
		}
		if groupStartIdx == -1 { // first time we add a layer (could be i==0 pick)
			groupStartIdx = i
		}
	}
	// remember to handle the leftover items in layersToSquash
	if len(layersToSquash) > 0 {
		if err := flushGroup(); err != nil {
			return newLayers, err
		}
	}
	return newLayers, nil
}

func (r *Runtime) squashLayers(ctx context.Context, parent Snapshot, layersToSquash []Layer) (Layer, error) {
	newLayer, _, err := r.createSnapshot(ctx, parent, layersToSquash)
	if err != nil {
		return Layer{}, fmt.Errorf("failed to apply layers to snapshot: %w", err)
	}
	return newLayer, nil
}
