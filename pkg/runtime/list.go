// Most of the code in this file is adapted from containerd/nerdctl.
package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/nerdctl/pkg/formatter"
	"github.com/containerd/nerdctl/pkg/imgutil"
	"github.com/containerd/platforms"
	"github.com/lingdie/image-manip-server/pkg/api/types"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type imageAttr struct {
	CreatedAt time.Time
	Digest    digest.Digest
	ID        string
	Name      string
	Size      int64
	PSA       platformSpecificAttr
	Image     images.Image
}

type platformSpecificAttr struct {
	Platform   v1.Platform
	Config     v1.Descriptor
	BlobSize   int64
	Repository string
	Tag        string
}

// ListImages prints images with columns: REPOSITORY, TAG, IMAGE ID, CREATED, PLATFORM, SIZE, BLOB SIZE
func (r *Runtime) ListImages(ctx context.Context, opts types.ImageListOptions) error {
	imageList, err := r.imagestore.List(ctx, opts.NameAndRefFilter...)
	if err != nil {
		return err
	}
	if len(opts.Filters) > 0 {
		imageList, err = r.filterImages(ctx, imageList, opts.Filters)
		if err != nil {
			return err
		}
	}
	imageAttrList, err := r.GetImageAttrList(ctx, imageList, opts.Names)
	if err != nil {
		return err
	}
	handleSortBy(imageAttrList, opts.SortBy)
	return r.printImages(imageAttrList, opts)
}

// Supported filters:
// - before=<image>[:<tag>]: Images created before given image (exclusive)
// - since=<image>[:<tag>]: Images created after given image (exclusive)
// - label=<key>[=<value>]: Matches images based on the presence of a label alone or a label and a value
// - dangling=true: Filter images by dangling
// - reference=<image>[:<tag>]: Filter images by reference (Matches both docker compatible wildcard pattern and regexp)
// - size=<operator><size>[<unit>]: Filter images based on size. Supported operators: >, >=, <, <=, =, ==. Supported units (case-insensitive): B, KB, KiB, MB, MiB, GB, GiB, TB, TiB. If unit is omitted, bytes are assumed.
func (r *Runtime) filterImages(ctx context.Context, imageList []images.Image, filters []string) ([]images.Image, error) {
	f, err := ParseFilters(filters)
	if err != nil {
		return nil, err
	}

	if f.Dangling != nil {
		imageList = FilterDangling(imageList, *f.Dangling)
	}

	imageList, err = FilterByLabel(ctx, r.client, imageList, f.Labels)
	if err != nil {
		return nil, err
	}

	imageList, err = FilterByReference(imageList, f.Reference)
	if err != nil {
		return nil, err
	}
	imageList, err = FilterBySize(ctx, r.client, r.snapshotter, imageList, f.Size)
	if err != nil {
		return nil, err
	}

	var beforeImages []images.Image
	if len(f.Before) > 0 {
		beforeImages, err = r.imagestore.List(ctx, f.Before...)
		if err != nil {
			return nil, err
		}
	}
	var sinceImages []images.Image
	if len(f.Since) > 0 {
		sinceImages, err = r.imagestore.List(ctx, f.Since...)
		if err != nil {
			return nil, err
		}
	}
	imageList = FilterImages(imageList, beforeImages, sinceImages)
	return imageList, nil
}

func handleSortBy(imageAttrList []imageAttr, sortBy string) {
	switch sortBy {
	case "", "none":
		// do nothing
	case "created":
		// newer first; zero times go last
		sort.Slice(imageAttrList, func(i, j int) bool {
			ti, tj := imageAttrList[i].CreatedAt, imageAttrList[j].CreatedAt
			if ti.IsZero() && tj.IsZero() {
				return imageAttrList[i].Name < imageAttrList[j].Name
			}
			if ti.IsZero() {
				return false
			}
			if tj.IsZero() {
				return true
			}
			return ti.After(tj)
		})
	case "size":
		// larger first; unknown (-1) goes last
		sort.Slice(imageAttrList, func(i, j int) bool {
			si, sj := imageAttrList[i].Size, imageAttrList[j].Size
			if si == sj {
				return imageAttrList[i].Name < imageAttrList[j].Name
			}
			if si < 0 {
				return false
			}
			if sj < 0 {
				return true
			}
			return si > sj
		})
	default:
		// do nothing
	}
}

func (r *Runtime) GetImageAttrList(ctx context.Context, imageList []images.Image, showOnlyNames bool) ([]imageAttr, error) {
	var imageAttrList []imageAttr
	for _, containerImage := range imageList {
		clientImage := containerd.NewImage(r.client, containerImage)
		size, err := imgutil.UnpackedImageSize(ctx, r.snapshotter, clientImage)
		if err != nil {
			r.Warnf("failed to get unpacked size of image %q: %v", containerImage.Name, err)
			size = 0
		}
		ociPlatforms, err := images.Platforms(ctx, r.contentstore, containerImage.Target)
		if err != nil {
			r.Warnf("failed to get the platform list of image %q: %v", containerImage.Name, err)
			ociPlatforms = []v1.Platform{platforms.DefaultSpec()}
		}
		psm := map[string]struct{}{}
		for _, ociPlatform := range ociPlatforms {
			platformKey := makePlatformKey(ociPlatform)
			if _, done := psm[platformKey]; done {
				continue
			}
			psm[platformKey] = struct{}{}
			if avail, _, _, _, availErr := images.Check(ctx, r.contentstore, containerImage.Target, platforms.OnlyStrict(ociPlatform)); !avail {
				r.Debugf("skipping image %q: %v", containerImage.Name, availErr)
				continue
			}
			blobSize, err := clientImage.Size(ctx)
			if err != nil {
				r.Warnf("failed to get blob size of image %q for platform %q: %v", containerImage.Name, platforms.Format(ociPlatform), err)
				blobSize = 0
			}
			configDesc, err := clientImage.Config(ctx)
			if err != nil {
				r.Warnf("failed to get config descriptor of image %q for platform %q: %v", containerImage.Name, platforms.Format(ociPlatform), err)
				configDesc = v1.Descriptor{}
			}
			var (
				repository string
				tag        string
			)
			// cri plugin will create an image named digest of image's config, skip parsing.
			// But if --names is specified, always show the name without parsing.
			if showOnlyNames || configDesc.Digest.String() != containerImage.Name {
				repository, tag = imgutil.ParseRepoTag(containerImage.Name)
			}
			imgAttr := imageAttr{
				CreatedAt: containerImage.CreatedAt,
				Digest:    containerImage.Target.Digest,
				ID:        containerImage.Target.Digest.String(),
				Name:      containerImage.Name,
				Image:     containerImage,
				Size:      size,
				PSA: platformSpecificAttr{
					BlobSize:   blobSize,
					Platform:   ociPlatform,
					Config:     configDesc,
					Repository: repository,
					Tag:        tag,
				},
			}
			imageAttrList = append(imageAttrList, imgAttr)
		}
	}
	return imageAttrList, nil
}

type imagePrintable struct {
	// TODO: "Containers"
	CreatedAt    string
	CreatedSince string
	Digest       string // "<none>" or image target digest (i.e., index digest or manifest digest)
	ID           string // image target digest (not config digest, unlike Docker), or its short form
	Repository   string
	Tag          string // "<none>" or tag
	Name         string // image name
	Size         string // the size of the unpacked snapshots.
	BlobSize     string // the size of the blobs in the content store (nerdctl extension)
	// TODO: "SharedSize", "UniqueSize"
	Platform string // nerdctl extension
}

func (r *Runtime) printImages(imageAttrList []imageAttr, options types.ImageListOptions) error {
	w := options.Stdout
	digestsFlag := options.Digests
	if options.Format == "wide" {
		digestsFlag = true
	}
	var tmpl *template.Template
	switch options.Format {
	case "", "table", "wide":
		w = tabwriter.NewWriter(w, 4, 8, 4, ' ', 0)
		if !options.Quiet {
			printHeader := ""
			if options.Names {
				printHeader += "NAME\t"
			} else {
				printHeader += "REPOSITORY\tTAG\t"
			}
			if digestsFlag {
				printHeader += "DIGEST\t"
			}
			printHeader += "IMAGE ID\tCREATED\tPLATFORM\tSIZE\tBLOB SIZE"
			fmt.Fprintln(w, printHeader)
		}
	case "raw":
		return errors.New("unsupported format: \"raw\"")
	default:
		if options.Quiet {
			return errors.New("format and quiet must not be specified together")
		}
		var err error
		tmpl, err = formatter.ParseTemplate(options.Format)
		if err != nil {
			return err
		}
	}

	printer := &imagePrinter{
		w:           w,
		quiet:       options.Quiet,
		noTrunc:     options.NoTrunc,
		digestsFlag: digestsFlag,
		namesFlag:   options.Names,
		tmpl:        tmpl,
	}

	for _, imgAttr := range imageAttrList {
		if err := printer.printImageSinglePlatform(imgAttr); err != nil {
			r.Warn(err)
		}
	}
	if f, ok := w.(formatter.Flusher); ok {
		return f.Flush()
	}
	return nil
}

type imagePrinter struct {
	w                                      io.Writer
	quiet, noTrunc, digestsFlag, namesFlag bool
	tmpl                                   *template.Template
}

func makePlatformKey(platform v1.Platform) string {
	if platform.OS == "" {
		return "unknown"
	}

	return path.Join(platform.OS, platform.Architecture, platform.OSVersion, platform.Variant)
}

func (x *imagePrinter) printImageSinglePlatform(imgAttr imageAttr) error {
	p := imagePrintable{
		CreatedAt:    imgAttr.CreatedAt.Round(time.Second).Local().String(), // format like "2021-08-07 02:19:45 +0900 JST"
		CreatedSince: formatter.TimeSinceInHuman(imgAttr.CreatedAt),
		Digest:       imgAttr.Digest.String(),
		ID:           imgAttr.ID,
		Repository:   imgAttr.PSA.Repository,
		Tag:          imgAttr.PSA.Tag,
		Name:         imgAttr.Name,
		Size:         progress.Bytes(imgAttr.Size).String(),
		BlobSize:     progress.Bytes(imgAttr.PSA.BlobSize).String(),
		Platform:     platforms.Format(imgAttr.PSA.Platform),
	}
	if p.Repository == "" {
		p.Repository = "<none>"
	}
	if p.Tag == "" {
		p.Tag = "<none>" // for Docker compatibility
	}
	if !x.noTrunc {
		// p.Digest does not need to be truncated
		p.ID = strings.Split(p.ID, ":")[1][:12]
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
		if _, err := fmt.Fprintln(x.w, p.ID); err != nil {
			return err
		}
	} else {
		format := ""
		args := []interface{}{}
		if x.namesFlag {
			format += "%s\t"
			args = append(args, p.Name)
		} else {
			format += "%s\t%s\t"
			args = append(args, p.Repository, p.Tag)
		}
		if x.digestsFlag {
			format += "%s\t"
			args = append(args, p.Digest)
		}

		format += "%s\t%s\t%s\t%s\t%s\n"
		args = append(args, p.ID, p.CreatedSince, p.Platform, p.Size, p.BlobSize)
		if _, err := fmt.Fprintf(x.w, format, args...); err != nil {
			return err
		}
	}
	return nil
}
