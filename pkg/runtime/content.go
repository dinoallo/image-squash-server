package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// WriteImageMetadata writes the image config and manifest to the content store and returns the manifest descriptor.
func (r *Runtime) writeImageMetadata(ctx context.Context, config ocispec.Image, layers []ocispec.Descriptor) (ocispec.Descriptor, error) {
	// write image contents to content store
	configDesc, err := r.writeImageConfig(ctx, config)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	manifestDesc, err := r.writeImageManifest(ctx, configDesc, layers)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return manifestDesc, nil
}

func createImageManifest(manifestJSON []byte) ocispec.Descriptor {
	manifestDesc := ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2Manifest,
		Digest:    digest.FromBytes(manifestJSON),
		Size:      int64(len(manifestJSON)),
	}
	return manifestDesc
}

func (r *Runtime) writeImageManifest(ctx context.Context, configDesc ocispec.Descriptor, layers []ocispec.Descriptor) (ocispec.Descriptor, error) {
	manifest := struct {
		MediaType string `json:"mediaType,omitempty"`
		ocispec.Manifest
	}{
		MediaType: images.MediaTypeDockerSchema2Manifest,
		Manifest: ocispec.Manifest{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			Config: configDesc,
			Layers: layers,
		},
	}

	manifestJSON, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	manifestDesc := createImageManifest(manifestJSON)

	// new manifest should reference the layers and config content
	labels := map[string]string{
		"containerd.io/gc.ref.content.0": configDesc.Digest.String(),
	}
	for i, l := range layers {
		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i+1)] = l.Digest.String()
	}

	err = content.WriteBlob(ctx, r.contentstore, manifestDesc.Digest.String(), bytes.NewReader(manifestJSON), manifestDesc, content.WithLabels(labels))
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return manifestDesc, nil
}

func createImageConfig(configJSON []byte) ocispec.Descriptor {
	configDesc := ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2Config,
		Digest:    digest.FromBytes(configJSON),
		Size:      int64(len(configJSON)),
	}
	return configDesc
}

func (r *Runtime) writeImageConfig(ctx context.Context, config ocispec.Image) (ocispec.Descriptor, error) {
	// marshal config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	// create config descriptor
	configDesc := createImageConfig(configJSON)
	snapshot := identity.ChainID(config.RootFS.DiffIDs).String()
	// there should be a reference from image to snapshot in the config
	labelOpt := content.WithLabels(map[string]string{
		fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", r.snapshotterName): snapshot,
	})
	if err := content.WriteBlob(ctx, r.contentstore, configDesc.Digest.String(), bytes.NewReader(configJSON), configDesc, labelOpt); err != nil {
		return ocispec.Descriptor{}, err
	}
	return configDesc, nil
}

func (r *Runtime) WriteBack(ctx context.Context, baseConfig ocispec.Image, baseLayers []ocispec.Descriptor, newLayers LayerChain) (ocispec.Descriptor, error) {
	// generate image config
	imageConfig, err := r.GenerateMergedImageConfig(ctx, baseConfig, newLayers)
	if err != nil {
		r.logger.Errorf("failed to generate new image config: %v", err)
		return ocispec.Descriptor{}, err
	}
	allLayers, err := NewLayerChain(
		baseLayers,
		baseConfig.RootFS.DiffIDs,
	)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	allLayers.AppendLayers(newLayers)
	// write image metadata
	manifestDesc, err := r.writeImageMetadata(ctx, imageConfig, allLayers.Descriptors)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return manifestDesc, nil
}
