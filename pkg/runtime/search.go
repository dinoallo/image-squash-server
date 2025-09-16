package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd/pkg/progress"
	"github.com/docker/go-units"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type historyPrintable struct {
	Snapshot     string
	CreatedSince string
	CreatedBy    string
	Size         string
	Comment      string
	LastSnapshot string
}

// ImageHistory returns the history entries for the given image reference.
func (r *Runtime) ImageHistory(ctx context.Context, imageRef string) ([]digest.Digest, []ocispec.History, error) {
	img, err := r.GetImage(ctx, imageRef)
	if err != nil {
		return nil, nil, err
	}
	return img.Config.RootFS.DiffIDs, img.Config.History, nil
}

func (r *Runtime) SearchImageHistory(ctx context.Context, opts options.SearchHistoryOptions) error {
	diffIDs, histories, err := r.ImageHistory(ctx, opts.ImageRef)
	if err != nil {
		return err
	}
	layerIndex := 0
	lastSnapshotName := ""
	var matched []historyPrintable
	for _, h := range histories {
		var (
			size         string
			snapshotName string
		)
		if len(diffIDs) <= layerIndex {
			break
		}
		if !h.EmptyLayer {
			chainID := identity.ChainID(diffIDs[0 : layerIndex+1]).String()
			stat, err := r.snapshotter.Stat(ctx, chainID)
			if err != nil {
				return fmt.Errorf("failed to get stat: %w", err)
			}
			use, err := r.snapshotter.Usage(ctx, chainID)
			if err != nil {
				return fmt.Errorf("failed to get usage: %w", err)
			}
			size = progress.Bytes(use.Size).String()
			snapshotName = stat.Name
			lastSnapshotName = snapshotName
			layerIndex++
		} else {
			size = progress.Bytes(0).String()
			snapshotName = "<missing>"
		}
		if strings.Contains(strings.ToLower(h.CreatedBy), strings.ToLower(opts.Keyword)) {
			history := historyPrintable{
				Snapshot:     snapshotName,
				CreatedSince: TimeSinceInHuman(*h.Created),
				CreatedBy:    h.CreatedBy,
				Size:         size,
				Comment:      h.Comment,
				LastSnapshot: lastSnapshotName,
			}
			matched = append(matched, history)
		}
	}
	return printMatchedHistory(matched)
}

func printMatchedHistory(matched []historyPrintable) error {
	for i := len(matched) - 1; i >= 0; i-- {
		h := matched[i]
		fmt.Printf("SNAPSHOT: %s, LAST SNAPSHOT: %s, BY: %s, CREATED: %s, SIZE: %s, COMMENT: %s\n",
			h.Snapshot, h.LastSnapshot, h.CreatedBy, h.CreatedSince, h.Size, h.Comment)
	}
	return nil
}

func TimeSinceInHuman(since time.Time) string {
	return fmt.Sprintf("%s ago", units.HumanDuration(time.Since(since)))
}
