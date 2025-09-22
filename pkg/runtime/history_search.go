package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/log"
	"github.com/containerd/nerdctl/pkg/formatter"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type historyPrintable struct {
	// LastSnapshot is the last snapshot name
	LastSnapshot string
	// LastLayer is the last non-empty layer's descriptor digest
	LastLayer    string
	CreatedSince string
	CreatedBy    string
	Size         string
	Comment      string
	Empty        bool
}

// ImageHistory returns the history entries for the given image reference.
func (r *Runtime) ImageHistory(ctx context.Context, imageRef string) (LayerChain, []ocispec.History, error) {
	img, err := r.GetImage(ctx, imageRef)
	if err != nil {
		return LayerChain{}, nil, err
	}
	layers, err := NewLayerChain(img.Manifest.Layers, img.Config.RootFS.DiffIDs)
	if err != nil {
		return LayerChain{}, nil, err
	}
	return layers, img.Config.History, nil
}

func (r *Runtime) CommentContains(ctx context.Context, imageRef string, pattern string) ([]digest.Digest, error) {
	var digests []digest.Digest
	layers, histories, err := r.ImageHistory(ctx, imageRef)
	if err != nil {
		return digests, err
	}
	layerIndex := len(layers.Descriptors) - 1
	for i := len(histories) - 1; i >= 0; i-- {
		h := histories[i]
		if h.EmptyLayer {
			continue
		}
		if strings.Contains(h.Comment, pattern) {
			// Found a matching comment
			digests = append(digests, layers.Descriptors[layerIndex].Digest)
		}
		layerIndex--
	}
	return digests, nil
}

func (r *Runtime) ListImageHistory(ctx context.Context, opts options.HistoryOptions) error {
	layers, histories, err := r.ImageHistory(ctx, opts.ImageRef)
	if err != nil {
		return err
	}
	if len(layers.Descriptors) != len(layers.DiffIDs) {
		return fmt.Errorf("invalid image: number of layers descriptors and diff IDs do not match")
	}
	layerLen := len(layers.DiffIDs)
	layerIndex := 0
	lastSnapshotName := ""
	lastLayerName := ""
	var all []historyPrintable
	for _, h := range histories {
		var (
			size  string
			empty bool
		)
		if layerLen <= layerIndex {
			break
		}
		if !h.EmptyLayer {
			chainID := identity.ChainID(layers.DiffIDs[0 : layerIndex+1]).String()
			stat, err := r.snapshotter.Stat(ctx, chainID)
			if err != nil {
				return fmt.Errorf("failed to get stat: %w", err)
			}
			use, err := r.snapshotter.Usage(ctx, chainID)
			if err != nil {
				return fmt.Errorf("failed to get usage: %w", err)
			}
			size = progress.Bytes(use.Size).String()
			lastSnapshotName = stat.Name
			lastLayerName = layers.Descriptors[layerIndex].Digest.String()
			layerIndex++
		} else {
			size = progress.Bytes(0).String()
			empty = true
		}
		history := historyPrintable{
			LastSnapshot: lastSnapshotName,
			LastLayer:    lastLayerName,
			CreatedSince: formatter.TimeSinceInHuman(*h.Created),
			CreatedBy:    h.CreatedBy,
			Size:         size,
			Comment:      h.Comment,
			Empty:        empty,
		}
		all = append(all, history)
	}
	return printHistory(all, opts)
}

// SearchImageHistory searches the image history for entries matching the given keyword.
// TODO: merge this function with ListImageHistory
func (r *Runtime) SearchImageHistory(ctx context.Context, opts options.SearchHistoryOptions) error {
	layers, histories, err := r.ImageHistory(ctx, opts.ImageRef)
	if err != nil {
		return err
	}
	if len(layers.Descriptors) != len(layers.DiffIDs) {
		return fmt.Errorf("invalid image: number of layers descriptors and diff IDs do not match")
	}
	layerLen := len(layers.DiffIDs)
	layerIndex := 0
	lastSnapshotName := ""
	lastLayerName := ""
	var matched []historyPrintable
	for _, h := range histories {
		var (
			size  string
			empty bool
		)
		if layerLen <= layerIndex {
			break
		}
		if !h.EmptyLayer {
			chainID := identity.ChainID(layers.DiffIDs[0 : layerIndex+1]).String()
			stat, err := r.snapshotter.Stat(ctx, chainID)
			if err != nil {
				return fmt.Errorf("failed to get stat: %w", err)
			}
			use, err := r.snapshotter.Usage(ctx, chainID)
			if err != nil {
				return fmt.Errorf("failed to get usage: %w", err)
			}
			size = progress.Bytes(use.Size).String()
			lastSnapshotName = stat.Name
			lastLayerName = layers.Descriptors[layerIndex].Digest.String()
			layerIndex++
		} else {
			size = progress.Bytes(0).String()
			empty = true
		}
		if strings.Contains(strings.ToLower(h.CreatedBy), strings.ToLower(opts.Keyword)) {
			history := historyPrintable{
				LastSnapshot: lastSnapshotName,
				LastLayer:    lastLayerName,
				CreatedSince: formatter.TimeSinceInHuman(*h.Created),
				CreatedBy:    h.CreatedBy,
				Size:         size,
				Comment:      h.Comment,
				Empty:        empty,
			}
			matched = append(matched, history)
		}
	}
	return printHistory(matched, opts.HistoryOptions)
}

/*
	func printMatchedHistory(matched []historyPrintable, opts options.SearchHistoryOptions) error {
		for i := len(matched) - 1; i >= 0; i-- {
			h := matched[i]
			fmt.Printf("SNAPSHOT: %s, LAST SNAPSHOT: %s, BY: %s, CREATED: %s, SIZE: %s, COMMENT: %s\n",
				h.Snapshot, h.LastSnapshot, h.CreatedBy, h.CreatedSince, h.Size, h.Comment)
		}
		return nil
	}
*/

type historyPrinter struct {
	w              io.Writer
	quiet, noTrunc bool
	tmpl           *template.Template
}

func printHistory(histories []historyPrintable, opts options.HistoryOptions) error {
	var tmpl *template.Template
	format := opts.Format
	quiet := opts.Quiet
	noTrunc := opts.NoTrunc
	var w io.Writer
	w = os.Stdout
	switch format {
	case "", "table":
		w = tabwriter.NewWriter(w, 4, 8, 4, ' ', 0)
		if !quiet {
			fmt.Fprintln(w, "LAST SNAPSHOT\tLAST LAYER\tEMPTY\tCREATED\tCREATED BY\tSIZE\tCOMMENT")
		}
	case "raw":
		return errors.New("unsupported format: \"raw\"")
	default:
		if quiet {
			return errors.New("format and quiet must not be specified together")
		}
		var err error
		tmpl, err = formatter.ParseTemplate(format)
		if err != nil {
			return err
		}
	}

	printer := &historyPrinter{
		w:       w,
		quiet:   quiet,
		noTrunc: noTrunc,
		tmpl:    tmpl,
	}

	for index := len(histories) - 1; index >= 0; index-- {
		if err := printer.printHistory(histories[index]); err != nil {
			log.L.Warn(err)
		}
	}

	if f, ok := w.(formatter.Flusher); ok {
		return f.Flush()
	}
	return nil
}

func (x *historyPrinter) printHistory(p historyPrintable) error {
	if !x.noTrunc {
		if len(p.CreatedBy) > 45 {
			p.CreatedBy = p.CreatedBy[0:44] + "…"
		}
	}
	if x.tmpl != nil {
		var b bytes.Buffer
		if err := x.tmpl.Execute(&b, p); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(x.w, b.String()); err != nil {
			return err
		}
	} else if x.quiet {
		if _, err := fmt.Fprintf(x.w, "%s\t%s\n",
			p.LastSnapshot,
			p.LastLayer); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(x.w, "%s\t%s\t%t\t%s\t%s\t%s\t%s\n",
			p.LastSnapshot,
			p.LastLayer,
			p.Empty,
			p.CreatedSince,
			p.CreatedBy,
			p.Size,
			p.Comment,
		); err != nil {
			return err
		}
	}
	return nil
}
