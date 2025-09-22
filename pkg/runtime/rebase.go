package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd/images"
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
	layers, err := NewLayerChain(image.Manifest.Layers, image.Config.RootFS.DiffIDs)
	if err != nil {
		return err
	}
	// generate the layers to be rebased
	baseLayerIndex, err := r.getBaseLayerIndex(layers, baseLayerDigest)
	if err != nil {
		r.Errorf("failed to generate layers to rebase: %v", err)
		return err
	}
	firstLayerIndexToRebase := baseLayerIndex + 1
	// if the base layer is the last layer, nothing to do
	if firstLayerIndexToRebase == layers.Len() {
		r.Infof("base layer digest %q is the last layer of the image, nothing to rebase", opt.BaseLayerDigest)
		return nil
	}
	// layers to be rebased
	layersToRebase, err := NewLayerChain(layers.Descriptors[firstLayerIndexToRebase:], layers.DiffIDs[firstLayerIndexToRebase:])
	if err != nil {
		r.Errorf("failed to create layer chain to rebase: %v", err)
		return err
	}

	root := NewSnapshot(image.Config.RootFS.DiffIDs[:firstLayerIndexToRebase])
	var rebaseToDoList []string
	if opt.AutoSquash {
		rebaseToDoList = getSquashAll(layersToRebase.Len())
	} else {
		rebaseToDoList = getAllPick(layersToRebase.Len())
	}
	// modify the layers according to the rebaseToDoList
	newLayers, err := r.modifyLayers(ctx, root, layersToRebase, rebaseToDoList)
	if err != nil {
		r.Errorf("failed to modify layers: %v", err)
		return err
	}
	// if NewBaseImageRef is specified, use the config and layers from the new base image
	// otherwise, use the config and layers from the original image up to the base layer
	// then append the new layers
	var baseConfig ocispec.Image
	var baseLayers LayerChain
	if opt.NewBaseImageRef != "" {
		newBaseImage, err := r.GetImage(ctx, opt.NewBaseImageRef)
		if err != nil {
			r.Errorf("failed to get new base image %q: %v", opt.NewBaseImageRef, err)
			return err
		}
		baseConfig = newBaseImage.Config
		baseLayers, err = NewLayerChain(newBaseImage.Manifest.Layers, newBaseImage.Config.RootFS.DiffIDs)
		if err != nil {
			r.Errorf("failed to create layer chain for new base image %q: %v", opt.NewBaseImageRef, err)
			return err
		}
	} else {
		baseConfig = image.Config
		baseLayers, err = NewLayerChain(image.Manifest.Layers[:firstLayerIndexToRebase], image.Config.RootFS.DiffIDs[:firstLayerIndexToRebase])
		if err != nil {
			r.Errorf("failed to create layer chain for original image %q: %v", opt.ImageRef, err)
			return err
		}
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

func (r *Runtime) getBaseLayerIndex(layerChain LayerChain, baseLayerRef digest.Digest) (int, error) {
	baseLayerIdx := -1
	//TODO: optimize this
	for i, l := range layerChain.Descriptors {
		if l.Digest == baseLayerRef {
			baseLayerIdx = i
			break
		}
	}
	if baseLayerIdx == -1 {
		return -1, fmt.Errorf("base layer %q not found", baseLayerRef)
	}
	return baseLayerIdx, nil
}

func (r *Runtime) modifyLayers(ctx context.Context, root Snapshot, layersToRebase LayerChain, rebaseToDoList []string) (LayerChain, error) {
	var (
		layersToSquash          = NewEmptyLayerChain()
		newLayers               = NewEmptyLayerChain()
		currentParent  Snapshot = root.Clone()
	)
	groupStartIdx := -1 // index in layersToRebase / rebaseDiffIDs where current group started

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
