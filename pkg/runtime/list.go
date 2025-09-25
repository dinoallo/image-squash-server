package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/nerdctl/pkg/formatter"
	"github.com/containerd/nerdctl/pkg/imgutil"
	"github.com/opencontainers/image-spec/identity"
)

type ImageSortKey string

const (
	SortNone    ImageSortKey = ""
	SortCreated ImageSortKey = "created"
	SortSize    ImageSortKey = "size"
)

type ListImagesOptions struct {
	SortBy ImageSortKey // "created" or "size"; default none
}

type imageRow struct {
	repo        string
	tag         string
	imageID     string
	createdStr  string
	createdAt   time.Time
	platform    string
	sizeStr     string
	sizeBytes   int64 // -1 if unknown
	blobSizeStr string
}

// ListImages prints images with columns: REPOSITORY, TAG, IMAGE ID, CREATED, PLATFORM, SIZE, BLOB SIZE
func (r *Runtime) ListImages(ctx context.Context, opts ...ListImagesOptions) error {
	// figure options
	var opt ListImagesOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// fetch all images
	imageList, err := r.imagestore.List(ctx)
	if err != nil {
		return err
	}

	rows := make([]imageRow, 0, len(imageList))

	for _, img := range imageList {
		repo, tag := parseRepoTag(img.Name)
		cimg := containerd.NewImage(r.client, img)

		row := imageRow{
			repo:        repo,
			tag:         tag,
			imageID:     "<unknown>",
			createdStr:  "<unknown>",
			platform:    "<unknown>",
			sizeStr:     "-",
			sizeBytes:   -1,
			blobSizeStr: "-",
		}

		manifest, _, err := imgutil.ReadManifest(ctx, cimg)
		if err == nil && manifest != nil {
			// BLOB SIZE: sum of compressed layer sizes
			var blobSize int64
			for _, l := range manifest.Layers {
				blobSize += l.Size
			}
			row.blobSizeStr = progress.Bytes(blobSize).String()
		}

		config, configDesc, err := imgutil.ReadImageConfig(ctx, cimg)
		if err == nil {
			// IMAGE ID: config digest short
			if configDesc.Digest != "" {
				d := configDesc.Digest.Encoded()
				if len(d) > 12 {
					d = d[:12]
				}
				row.imageID = d
			}
			// CREATED
			if config.Created != nil {
				row.createdAt = *config.Created
				row.createdStr = formatter.TimeSinceInHuman(*config.Created)
			}
			// PLATFORM
			arch := strings.TrimSpace(config.Architecture)
			os := strings.TrimSpace(config.OS)
			if arch != "" && os != "" {
				row.platform = fmt.Sprintf("%s/%s", arch, os)
			}
			// SIZE: try snapshot size of unpacked rootfs
			chainID := identity.ChainID(config.RootFS.DiffIDs).String()
			if chainID != "" {
				if usage, err := r.snapshotter.Usage(ctx, chainID); err == nil {
					row.sizeBytes = usage.Size
					row.sizeStr = progress.Bytes(usage.Size).String()
				}
			}
		}

		rows = append(rows, row)
	}

	// sort if requested (descending by default)
	switch opt.SortBy {
	case SortCreated:
		sort.Slice(rows, func(i, j int) bool {
			// newer first; zero times go last
			ti, tj := rows[i].createdAt, rows[j].createdAt
			if ti.IsZero() && tj.IsZero() {
				return rows[i].repo < rows[j].repo
			}
			if ti.IsZero() {
				return false
			}
			if tj.IsZero() {
				return true
			}
			return ti.After(tj)
		})
	case SortSize:
		sort.Slice(rows, func(i, j int) bool {
			// larger first; unknown (-1) goes last
			si, sj := rows[i].sizeBytes, rows[j].sizeBytes
			if si == sj {
				return rows[i].repo < rows[j].repo
			}
			if si < 0 {
				return false
			}
			if sj < 0 {
				return true
			}
			return si > sj
		})
	}

	tw := tabwriter.NewWriter(r.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "REPOSITORY\tTAG\tIMAGE ID\tCREATED\tPLATFORM\tSIZE\tBLOB SIZE")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.repo, row.tag, row.imageID, row.createdStr, row.platform, row.sizeStr, row.blobSizeStr,
		)
	}
	return tw.Flush()
}

func parseRepoTag(name string) (string, string) {
	if name == "" {
		return "<none>", "<none>"
	}
	// digest reference
	if i := strings.Index(name, "@"); i >= 0 {
		return name[:i], "<none>"
	}
	// tag reference, last colon after last slash
	lastSlash := strings.LastIndex(name, "/")
	lastColon := strings.LastIndex(name, ":")
	if lastColon > lastSlash && lastColon >= 0 {
		return name[:lastColon], name[lastColon+1:]
	}
	return name, "<none>"
}
