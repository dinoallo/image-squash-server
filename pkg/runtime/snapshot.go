package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	"github.com/lingdie/image-manip-server/pkg/util"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// createSnapshot creates a new snapshot from the parent specified by parentDiffIDs, and apply the given layers to it.
func (r *Runtime) createSnapshot(ctx context.Context, parentDiffIDs []digest.Digest, sn snapshots.Snapshotter, layers []ocispec.Descriptor) (
	ocispec.Descriptor, digest.Digest, string, error) {
	var (
		key          = util.UniquePart()
		parent       = identity.ChainID(parentDiffIDs).String()
		newLayerDesc ocispec.Descriptor
		diffID       digest.Digest
		snapshotID   string
		retErr       error
	)

	m, err := sn.Prepare(ctx, key, parent)
	if err != nil {
		return newLayerDesc, diffID, snapshotID, err
	}

	defer func() {
		if retErr != nil {
			// NOTE: the snapshotter should be hold by lease. Even
			// if the cleanup fails, the containerd gc can delete it.
			if err := sn.Remove(ctx, key); err != nil {
				r.logger.Warnf("failed to cleanup aborted apply %s: %v", key, err)
			}
		}
	}()
	for _, layer := range layers {
		err = r.applyLayerToMount(ctx, m, layer)
		if err != nil {
			r.logger.Warnf("failed to apply layer to mount %q: %v", m, err)
			return newLayerDesc, diffID, snapshotID, err
		}
	}
	newLayerDesc, diffID, err = r.createDiff(ctx, key)
	if err != nil {
		return newLayerDesc, diffID, snapshotID, fmt.Errorf("failed to export layer: %w", err)
	}

	// commit snapshot
	snapshotID = identity.ChainID(append(parentDiffIDs, diffID)).String()

	if err = sn.Commit(ctx, snapshotID, key); err != nil {
		if errdefs.IsAlreadyExists(err) {
			return newLayerDesc, diffID, snapshotID, nil
		}
		return newLayerDesc, diffID, snapshotID, err
	}
	return newLayerDesc, diffID, snapshotID, nil
}

func (r *Runtime) applyLayerToMount(ctx context.Context, mount []mount.Mount, layer ocispec.Descriptor) error {
	r.logger.Infof("apply layer %s to mount %q", layer.Digest, mount)
	start := time.Now()
	if _, err := r.differ.Apply(ctx, layer, mount); err != nil {
		return err
	}
	elapsed := time.Since(start)
	r.logger.Infof("apply layer %s to mount %q cost %s", layer.Digest, mount, elapsed)
	return nil
}

// createDiff creates a diff between a snapshot and its parent
func (r *Runtime) createDiff(ctx context.Context, snapshotName string) (ocispec.Descriptor, digest.Digest, error) {
	r.logger.Infof("create diff for snapshot %s", snapshotName)
	start := time.Now()

	// // Create a zstd compressor with high compression level
	// zstdCompressor := func(dest io.Writer, mediaType string) (io.WriteCloser, error) {
	// 	// Use zstd with high compression level for better compression ratio
	// 	// while maintaining good speed
	// 	encoder, err := zstd.NewWriter(dest, zstd.WithEncoderLevel(zstd.SpeedFastest))
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	return encoder, nil
	// }

	// Create diff with custom compressor
	// newDesc, err := rootfs.CreateDiff(ctx, snapshotName, r.snapshotter, r.differ, diff.WithCompressor(zstdCompressor))
	newDesc, err := rootfs.CreateDiff(ctx, snapshotName, r.snapshotter, r.differ)
	if err != nil {
		return ocispec.Descriptor{}, "", err
	}
	info, err := r.contentstore.Info(ctx, newDesc.Digest)
	if err != nil {
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	diffIDStr, ok := info.Labels["containerd.io/uncompressed"]
	if !ok {
		return ocispec.Descriptor{}, digest.Digest(""), fmt.Errorf("invalid differ response with no diffID")
	}
	diffID, err := digest.Parse(diffIDStr)
	if err != nil {
		return ocispec.Descriptor{}, digest.Digest(""), err
	}
	elapsed := time.Since(start)
	r.logger.Infof("create diff for snapshot %s cost %s", snapshotName, elapsed)
	return ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2LayerGzip,
		Digest:    newDesc.Digest,
		Size:      info.Size,
	}, diffID, nil
}
