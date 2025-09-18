package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd/images"
	imageutil "github.com/lingdie/image-manip-server/pkg/images"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (r *Runtime) Rebase(ctx context.Context, opt options.RebaseOptions) error {
	r.logger.Infof("start to rebase image %q to layer digest %q", opt.ImageRef, opt.BaseLayerDigest)
	defer r.Track(time.Now(), "rebase")
	// get the image to be rebased
	image, err := r.GetImage(ctx, opt.ImageRef)
	if err != nil {
		r.logger.Errorf("failed to get original image %q: %v", opt.ImageRef, err)
		return err
	}
	baseLayerDigest, err := digest.Parse(opt.BaseLayerDigest)
	if err != nil {
		r.logger.Errorf("failed to parse base layer ref %q: %v", baseLayerDigest, err)
		return err
	}
	// generate the layers to be rebased
	layersToRebase, baseLayerIndex, err := r.generateLayersToRebase(image, baseLayerDigest)
	if err != nil {
		r.logger.Errorf("failed to generate layers to rebase: %v", err)
		return err
	}
	root := NewSnapshot(image.Config.RootFS.DiffIDs[:baseLayerIndex])
	var rebaseToDoList []string
	if opt.AutoSquash {
		rebaseToDoList = getSquashAll(layersToRebase)
	} else {
		rebaseToDoList = getAllPick(layersToRebase)
	}
	// modify the layers according to the rebaseToDoList
	newLayers, err := r.modifyLayers(ctx, root, layersToRebase, rebaseToDoList)
	if err != nil {
		r.logger.Errorf("failed to modify layers: %v", err)
		return err
	}
	// if NewBaseImageRef is specified, use the config and layers from the new base image
	// otherwise, use the config and layers from the original image up to the base layer
	// then append the new layers
	var baseConfig ocispec.Image
	var baseLayers []ocispec.Descriptor
	if opt.NewBaseImageRef != "" {
		newBaseImage, err := r.GetImage(ctx, opt.NewBaseImageRef)
		if err != nil {
			r.logger.Errorf("failed to get new base image %q: %v", opt.NewBaseImageRef, err)
			return err
		}
		baseConfig = newBaseImage.Config
		baseLayers = newBaseImage.Manifest.Layers
	} else {
		baseConfig = image.Config
		baseLayers = image.Manifest.Layers[:baseLayerIndex]
	}
	// finally, write back the new image to the image store
	manifestDesc, err := r.WriteBack(ctx, baseConfig, baseLayers, newLayers)
	if err != nil {
		return err
	}
	var newImageName string
	// determine the new image name
	// if NewImageName is not specified, use the original image name
	if opt.NewImageName != "" {
		newImageName = opt.NewImageName
	} else {
		newImageName = image.Image.Name
	}
	img := images.Image{
		Name:      newImageName,
		Target:    manifestDesc,
		UpdatedAt: time.Now(),
	}
	// update the image in the image store
	img, err = r.UpdateImage(ctx, img)
	if err != nil {
		return err
	}
	// unpack image to the snapshot storage
	if err := r.UnpackImage(ctx, img, manifestDesc); err != nil {
		r.logger.Errorf("failed to unpack image %q: %v", img.Name, err)
		return err
	}
	r.logger.Infof("rebase image %q successfully, new image: %q", opt.ImageRef, img.Name)
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
		return newLayer, fmt.Errorf("failed to apply layers to snapshot: %w", err)
	}
	return newLayer, nil
}
