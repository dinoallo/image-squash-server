package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/rootfs"
	"github.com/lingdie/image-manip-server/pkg/util"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// The content of diffChain can be safely modified
// because we always make a copy when creating a new Snapshot
// The content of name should never be modified
type Snapshot struct {
	Name      string
	DiffChain []digest.Digest
}

func NewSnapshot(_diffChain []digest.Digest) Snapshot {
	// make a copy of _diffChain to avoid modification
	// of the original slice
	diffChain := make([]digest.Digest, len(_diffChain))
	copy(diffChain, _diffChain)
	return Snapshot{
		Name:      identity.ChainID(diffChain).String(),
		DiffChain: diffChain,
	}
}

func (s *Snapshot) NewChild(diffID digest.Digest) Snapshot {
	// make a copy of DiffChain to avoid modification
	// of the original slice
	childDiffChain := make([]digest.Digest, len(s.DiffChain), len(s.DiffChain)+1)
	copy(childDiffChain, s.DiffChain)
	childDiffChain[len(s.DiffChain)] = diffID
	return Snapshot{
		Name:      identity.ChainID(childDiffChain).String(),
		DiffChain: childDiffChain,
	}
}

func (s *Snapshot) Clone() Snapshot {
	// make a copy of DiffChain to avoid modification
	// of the original slice
	diffChain := make([]digest.Digest, len(s.DiffChain))
	copy(diffChain, s.DiffChain)
	return Snapshot{
		Name:      s.Name,
		DiffChain: diffChain,
	}
}

// TODO: should we just use rootfs.ApplyLayers?
// createSnapshot creates a new snapshot from the parent specified by parentDiffIDs, and apply the given layers to it.
func (r *Runtime) createSnapshot(ctx context.Context, parent Snapshot, layerChain LayerChain) (
	Layer, string, error) {
	var (
		key        = util.UniquePart()
		parentName = parent.Name
		newLayer   Layer
		snapshotID string
		retErr     error
		sn         = r.snapshotter
	)

	m, err := sn.Prepare(ctx, key, parentName)
	if err != nil {
		return newLayer, snapshotID, err
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
	for i := 0; i < layerChain.Len(); i++ {
		layer, err := layerChain.GetLayerByIndex(i)
		if err != nil {
			return newLayer, snapshotID, err
		}
		err = r.applyLayerToMount(ctx, m, layer.Desc)
		if err != nil {
			r.logger.Warnf("failed to apply layer to mount %q: %v", m, err)
			return newLayer, snapshotID, err
		}
	}
	// create diff
	newLayer, err = r.createDiff(ctx, key)
	if err != nil {
		return newLayer, snapshotID, fmt.Errorf("failed to export layer: %w", err)
	}
	// commit snapshot
	snapshotID = identity.ChainID(append(parent.DiffChain, newLayer.DiffID)).String()

	if err = sn.Commit(ctx, snapshotID, key); err != nil {
		if errdefs.IsAlreadyExists(err) {
			return newLayer, snapshotID, nil
		}
		return newLayer, snapshotID, err
	}
	return newLayer, snapshotID, nil
}

func (r *Runtime) applyLayerToMount(ctx context.Context, mount []mount.Mount, layer ocispec.Descriptor) error {
	defer r.Track(time.Now(), "applyLayerToMount")
	r.logger.Infof("apply layer %s to mount %q", layer.Digest, mount)
	if _, err := r.differ.Apply(ctx, layer, mount); err != nil {
		return err
	}
	return nil
}

// createDiff creates a diff between a snapshot and its parent
func (r *Runtime) createDiff(ctx context.Context, snapshotName string) (Layer, error) {
	defer r.Track(time.Now(), "createDiff")
	r.logger.Infof("create diff for snapshot %s", snapshotName)
	var (
		layer = NewLayer(ocispec.Descriptor{}, digest.Digest(""))
	)
	newDesc, err := rootfs.CreateDiff(ctx, snapshotName, r.snapshotter, r.differ)
	if err != nil {
		return layer, err
	}
	info, err := r.contentstore.Info(ctx, newDesc.Digest)
	if err != nil {
		return layer, err
	}
	diffIDStr, ok := info.Labels["containerd.io/uncompressed"]
	if !ok {
		return layer, fmt.Errorf("invalid differ response with no diffID")
	}
	diffID, err := digest.Parse(diffIDStr)
	if err != nil {
		return layer, err
	}
	layer.Desc = ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2LayerGzip,
		Digest:    newDesc.Digest,
		Size:      info.Size,
	}
	layer.DiffID = diffID
	r.logger.Infof("diff for snapshot %s created", snapshotName)
	return layer, nil
}
