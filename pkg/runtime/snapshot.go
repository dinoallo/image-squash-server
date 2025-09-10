package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/pkg/cleanup"
	"github.com/containerd/containerd/snapshots"
	"github.com/lingdie/image-manip-server/pkg/util"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// TODO: should we just use rootfs.ApplyLayers?
// createSnapshot creates a new snapshot from the parent specified by parentDiffIDs, and apply the given layers to it.
func (r *Runtime) createSnapshot(ctx context.Context, parentDiffIDs []digest.Digest, layers []ocispec.Descriptor) (
	ocispec.Descriptor, digest.Digest, string, error) {
	var (
		key          = util.UniquePart()
		newLayerDesc ocispec.Descriptor
		diffID       digest.Digest
		snapshotID   string
		ancestor     = identity.ChainID(parentDiffIDs).String()
		child        string
		sn           = r.snapshotter
	)
	for _, layer := range layers {
		child = identity.ChainID(append(parentDiffIDs, layer.Digest)).String()
	}

	newLayerDesc, diffID, err := r.createDiff(ctx, child, ancestor)
	if err != nil {
		return newLayerDesc, diffID, snapshotID, fmt.Errorf("failed to export layer: %w", err)
	}
	m, err := sn.Prepare(ctx, key, ancestor)
	if err != nil {
		return newLayerDesc, diffID, snapshotID, fmt.Errorf("failed to prepare snapshot: %w", err)
	}
	applyErr := r.applyLayerToMount(ctx, m, newLayerDesc)
	if applyErr != nil {
		sn.Remove(ctx, key)
		return newLayerDesc, diffID, snapshotID, fmt.Errorf("failed to apply layer to mount: %w", applyErr)
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

func (r *Runtime) createDiff(ctx context.Context, child, ancestor string) (ocispec.Descriptor, digest.Digest, error) {
	start := time.Now()
	newDesc, err := r.CreateDiff(ctx, child, ancestor, r.differ)
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
	r.logger.Infof("create diff cost %s", elapsed)
	return ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2LayerGzip,
		Digest:    newDesc.Digest,
		Size:      info.Size,
	}, diffID, nil
}

func (r *Runtime) CreateDiff(ctx context.Context, child, ancestor string, d diff.Comparer, opts ...diff.Opt) (ocispec.Descriptor, error) {
	var (
		sn = r.snapshotter
	)
	lowerKey := fmt.Sprintf("%s-ancestor-view-%s", ancestor, util.UniquePart())
	lower, err := sn.View(ctx, lowerKey, ancestor)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer cleanup.Do(ctx, func(ctx context.Context) {
		sn.Remove(ctx, lowerKey)
	})
	info, err := sn.Stat(ctx, child)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	var upper []mount.Mount
	if info.Kind == snapshots.KindActive {
		upper, err = sn.Mounts(ctx, child)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
	} else {
		upperKey := fmt.Sprintf("%s-child-view-%s", child, util.UniquePart())
		upper, err = sn.View(ctx, upperKey, child)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		defer cleanup.Do(ctx, func(ctx context.Context) {
			sn.Remove(ctx, upperKey)
		})
	}
	return d.Compare(ctx, upper, lower, opts...)
}
