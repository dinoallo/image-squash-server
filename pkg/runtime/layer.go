package runtime

import (
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Layers struct {
	// This is referred in image manifest
	Descriptors []ocispec.Descriptor
	// This is referred in image config
	DiffIDs []digest.Digest
}

func NewLayers(_descriptors []ocispec.Descriptor, _diffIDs []digest.Digest) Layers {
	descriptors := make([]ocispec.Descriptor, len(_descriptors))
	copy(descriptors, _descriptors)
	diffIDs := make([]digest.Digest, len(_diffIDs))
	copy(diffIDs, _diffIDs)
	return Layers{
		Descriptors: descriptors,
		DiffIDs:     diffIDs,
	}
}

func NewLayersFromLayer(layer Layer) Layers {
	return Layers{
		Descriptors: []ocispec.Descriptor{layer.Desc},
		DiffIDs:     []digest.Digest{layer.DiffID},
	}
}

func NewEmptyLayers() Layers {
	return Layers{
		Descriptors: []ocispec.Descriptor{},
		DiffIDs:     []digest.Digest{},
	}
}

func (l *Layers) AppendLayer(layer Layer) {
	l.Descriptors = append(l.Descriptors, layer.Desc)
	l.DiffIDs = append(l.DiffIDs, layer.DiffID)
}

func (l *Layers) AppendLayers(layers Layers) {
	//TODO: optimize memory allocation
	l.Descriptors = append(l.Descriptors, layers.Descriptors...)
	l.DiffIDs = append(l.DiffIDs, layers.DiffIDs...)
}

// Layer represents an image layer along with its DiffID
type Layer struct {
	// This is referred in image manifest
	Desc ocispec.Descriptor
	// An image layer DiffID is the digest over the image layer's uncompressed archive and serialized in the descriptor digest format
	DiffID digest.Digest
}

func NewLayer(desc ocispec.Descriptor, diffID digest.Digest) Layer {
	//TODO: deep copy?
	return Layer{
		Desc:   desc,
		DiffID: diffID,
	}
}
