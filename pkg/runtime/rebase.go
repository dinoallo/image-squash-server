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
	r.Infof("start to rebase image %q to layer digest %q", opt.ImageRef, opt.BaseLayerDigest)
	defer r.Track(time.Now(), "rebase")
	// get the image to be rebased
	image, err := r.GetImage(ctx, opt.ImageRef)
	if err != nil {
		r.Errorf("failed to get original image %q: %v", opt.ImageRef, err)
		return err
	}
	baseLayerDigest, err := digest.Parse(opt.BaseLayerDigest)
	if err != nil {
		r.Errorf("failed to parse base layer ref %q: %v", baseLayerDigest, err)
		return err
	}
	// generate the layers to be rebased
	layers, baseLayerIndex, err := r.generateLayersToRebase(image, baseLayerDigest)
	if err != nil {
		r.Errorf("failed to generate layers to rebase: %v", err)
		return err
	}
	if len(layers.Descriptors) != len(layers.DiffIDs) {
		return fmt.Errorf("invalid layers to rebase: number of descriptors (%d) does not match number of diff IDs (%d)", len(layers.Descriptors), len(layers.DiffIDs))
	}
	if baseLayerIndex == len(layers.Descriptors)-1 {
		r.Infof("base layer digest %q is the last layer of the image, nothing to rebase", opt.BaseLayerDigest)
		return nil
	}
	// layers to be rebased
	layersToRebase, err := NewLayerChain(layers.Descriptors[baseLayerIndex:], layers.DiffIDs[baseLayerIndex:])
	if err != nil {
		r.Errorf("failed to create layer chain to rebase: %v", err)
		return err
	}

	root := NewSnapshot(image.Config.RootFS.DiffIDs[:baseLayerIndex])
	var rebaseToDoList []string
	if opt.AutoSquash {
		rebaseToDoList = getSquashAll(layersToRebase.Len())
	} else {
		rebaseToDoList = getAllPick(layersToRebase.Len())
	}
	// modify the layers according to the rebaseToDoList
	newLayers, err := r.modifyLayers(ctx, root, layers, baseLayerIndex, rebaseToDoList)
	if err != nil {
		r.Errorf("failed to modify layers: %v", err)
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
			r.Errorf("failed to get new base image %q: %v", opt.NewBaseImageRef, err)
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
		r.Errorf("failed to unpack image %q: %v", img.Name, err)
		return err
	}
	r.Infof("rebase image %q successfully, new image: %q", opt.ImageRef, img.Name)
	return nil
}

func getSquashAll(layerLen int) []string {
	var toDoList []string
	for i := 0; i < layerLen; i++ {
		if i == 0 {
			toDoList = append(toDoList, "pick")
		} else {
			toDoList = append(toDoList, "fixup")
		}
	}
	return toDoList
}

func getAllPick(layerLen int) []string {
	toDoList := make([]string, layerLen)
	for i := 0; i < layerLen; i++ {
		toDoList[i] = "pick"
	}
	return toDoList
}

func (r *Runtime) generateLayersToRebase(origImage imageutil.Image, targetLayerRef digest.Digest) (LayerChain, int, error) {
	if len(origImage.Manifest.Layers) != len(origImage.Config.RootFS.DiffIDs) {
		return LayerChain{}, -1, fmt.Errorf("invalid image %q: number of layers in manifest (%d) does not match number of diff IDs in config (%d)", origImage.Image.Name, len(origImage.Manifest.Layers), len(origImage.Config.RootFS.DiffIDs))
	}
	// find the target layer index
	imageLayers, err := NewLayerChain(origImage.Manifest.Layers, origImage.Config.RootFS.DiffIDs)
	if err != nil {
		return LayerChain{}, -1, err
	}
	targetLayerIdx := -1
	//TODO: optimize this
	for i, l := range origImage.Manifest.Layers {
		if l.Digest == targetLayerRef {
			targetLayerIdx = i
			break
		}
	}
	if targetLayerIdx == -1 {
		return imageLayers, -1, fmt.Errorf("target layer %q not found in original image %q", targetLayerRef, origImage.Image.Name)
	}
	return imageLayers, targetLayerIdx, nil
}

func (r *Runtime) modifyLayers(ctx context.Context, root Snapshot, layers LayerChain, baseLayerIdx int, rebaseToDoList []string) (LayerChain, error) {
	var (
		layersToSquash          = NewEmptyLayerChain()
		newLayers               = NewEmptyLayerChain()
		currentParent  Snapshot = root.Clone()
	)
	groupStartIdx := -1 // index in layersToRebase / rebaseDiffIDs where current group started
	if baseLayerIdx == len(layers.DiffIDs) {
		// nothing to rebase
		return newLayers, nil
	}
	if baseLayerIdx > len(layers.DiffIDs) {
		return newLayers, fmt.Errorf("base layer index %d out of range, total layers %d", baseLayerIdx, len(layers.DiffIDs))
	}
	// layers to be rebased
	layersToRebase, err := NewLayerChain(layers.Descriptors[baseLayerIdx:], layers.DiffIDs[baseLayerIdx:])
	if err != nil {
		return newLayers, err
	}

	update := func(layer Layer) {
		newLayers.AppendLayer(layer)
		currentParent = currentParent.NewChild(layer.DiffID)
		layersToSquash.Clear()
	}

	flushGroup := func() error {
		if layersToSquash.IsEmpty() {
			return nil
		}
		if layersToSquash.Len() == 1 {
			// Reuse the original single layer & its diffID instead of re-squashing
			layer, err := layersToSquash.GetLayerByIndex(0)
			if err != nil {
				return err
			}
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
	for i, action := range rebaseToDoList {
		layer := NewLayer(layersToRebase.Descriptors[i], layersToRebase.DiffIDs[i])
		switch action {
		case "fixup":
			if i == 0 {
				// the first layer cannot be fixed up
				return newLayers, fmt.Errorf("the first layer cannot be fixed up")
			}
			layersToSquash.AppendLayer(layer)
		case "pick":
			if layersToSquash.Len() > 0 {
				if err := flushGroup(); err != nil {
					return newLayers, err
				}
			}
			layersToSquash.Clear()
			layersToSquash.AppendLayer(layer)
			groupStartIdx = i

		default:
			return newLayers, fmt.Errorf("unknown action %q", action)
		}
		if groupStartIdx == -1 { // first time we add a layer (could be i==0 pick)
			groupStartIdx = i
		}
	}
	// remember to handle the leftover items in layersToSquash
	if layersToSquash.Len() > 0 {
		if err := flushGroup(); err != nil {
			return newLayers, err
		}
	}
	return newLayers, nil
}

func (r *Runtime) squashLayers(ctx context.Context, parent Snapshot, layersToSquash LayerChain) (Layer, error) {
	newLayer, _, err := r.createSnapshot(ctx, parent, layersToSquash)
	if err != nil {
		return newLayer, fmt.Errorf("failed to apply layers to snapshot: %w", err)
	}
	return newLayer, nil
}
