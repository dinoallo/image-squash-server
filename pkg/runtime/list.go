package runtime

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/nerdctl/pkg/formatter"
	"github.com/containerd/nerdctl/pkg/imgutil"
	"github.com/opencontainers/image-spec/identity"
)

// ListImages prints images with columns: REPOSITORY, TAG, IMAGE ID, CREATED, PLATFORM, SIZE, BLOB SIZE
func (r *Runtime) ListImages(ctx context.Context) error {
	// fetch all images
	imageList, err := r.imagestore.List(ctx)
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(r.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "REPOSITORY\tTAG\tIMAGE ID\tCREATED\tPLATFORM\tSIZE\tBLOB SIZE")

	for _, img := range imageList {
		repo, tag := parseRepoTag(img.Name)
		cimg := containerd.NewImage(r.client, img)

		// Defaults
		imageID := "<unknown>"
		created := "<unknown>"
		platform := "<unknown>"
		sizeStr := "-"
		blobSizeStr := "-"

		manifest, _, err := imgutil.ReadManifest(ctx, cimg)
		if err == nil && manifest != nil {
			// BLOB SIZE: sum of compressed layer sizes
			var blobSize int64
			for _, l := range manifest.Layers {
				blobSize += l.Size
			}
			blobSizeStr = progress.Bytes(blobSize).String()
		}

		config, configDesc, err := imgutil.ReadImageConfig(ctx, cimg)
		if err == nil {
			// IMAGE ID: config digest short
			if configDesc.Digest != "" {
				d := configDesc.Digest.Encoded()
				if len(d) > 12 {
					d = d[:12]
				}
				imageID = d
			}
			// CREATED
			if config.Created != nil {
				created = formatter.TimeSinceInHuman(*config.Created)
			}
			// PLATFORM
			arch := strings.TrimSpace(config.Architecture)
			os := strings.TrimSpace(config.OS)
			if arch != "" && os != "" {
				platform = fmt.Sprintf("%s/%s", arch, os)
			}
			// SIZE: try snapshot size of unpacked rootfs
			chainID := identity.ChainID(config.RootFS.DiffIDs).String()
			if chainID != "" {
				if usage, err := r.snapshotter.Usage(ctx, chainID); err == nil {
					sizeStr = progress.Bytes(usage.Size).String()
				}
			}
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			repo, tag, imageID, created, platform, sizeStr, blobSizeStr,
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
