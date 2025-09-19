package runtime

import (
	"fmt"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// The items of descriptors and diffIDs can be safely appended, removed, modified (excluding the content of each item)
// because we always make a copy when creating a new LayerChain
type LayerChain struct {
	// This is referred in image manifest
	Descriptors []ocispec.Descriptor
	// This is referred in image config
	DiffIDs []digest.Digest
}

func NewLayerChain(_descriptors []ocispec.Descriptor, _diffIDs []digest.Digest) (LayerChain, error) {
	if len(_descriptors) != len(_diffIDs) {
		return LayerChain{}, fmt.Errorf("number of descriptors (%d) does not match number of diff IDs (%d)", len(_descriptors), len(_diffIDs))
	}
	descriptors := make([]ocispec.Descriptor, len(_descriptors))
	copy(descriptors, _descriptors)
	diffIDs := make([]digest.Digest, len(_diffIDs))
	copy(diffIDs, _diffIDs)
	return LayerChain{
		Descriptors: descriptors,
		DiffIDs:     diffIDs,
	}, nil
}

func NewLayerChainFromLayer(layer Layer) LayerChain {
	return LayerChain{
		Descriptors: []ocispec.Descriptor{layer.Desc},
		DiffIDs:     []digest.Digest{layer.DiffID},
	}
}

func NewEmptyLayerChain() LayerChain {
	return LayerChain{
		Descriptors: []ocispec.Descriptor{},
		DiffIDs:     []digest.Digest{},
	}
}

func (l *LayerChain) Len() int {
	return len(l.Descriptors)
}

func (l *LayerChain) IsEmpty() bool {
	return l.Len() == 0
}

func (l *LayerChain) Clear() {
	l.Descriptors = l.Descriptors[:0]
	l.DiffIDs = l.DiffIDs[:0]
}

func (l *LayerChain) AppendLayers(layers LayerChain) {
	//TODO: optimize memory allocation
	l.Descriptors = append(l.Descriptors, layers.Descriptors...)
	l.DiffIDs = append(l.DiffIDs, layers.DiffIDs...)
}

func (l *LayerChain) AppendLayer(layer Layer) {
	//TODO: optimize memory allocation
	l.Descriptors = append(l.Descriptors, layer.Desc)
	l.DiffIDs = append(l.DiffIDs, layer.DiffID)
}

func (l *LayerChain) GetLayerByIndex(index int) (Layer, error) {
	if index < 0 || index >= l.Len() {
		return Layer{}, fmt.Errorf("index %d out of range, layer chain length is %d", index, l.Len())
	}
	return Layer{
		Desc:   l.Descriptors[index],
		DiffID: l.DiffIDs[index],
	}, nil
}

// The content of Desc and DiffID can not be modified
type Layer struct {
	Desc   ocispec.Descriptor
	DiffID digest.Digest
}

func NewLayer(desc ocispec.Descriptor, diffID digest.Digest) Layer {
	return Layer{
		Desc:   desc,
		DiffID: diffID,
	}
}

func NewEmptyLayer() Layer {
	return Layer{}
}
