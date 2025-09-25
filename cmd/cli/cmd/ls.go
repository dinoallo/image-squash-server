package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/api/types"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdLs() *cobra.Command {
	shortHelp := "List images"
	longHelp := shortHelp + `

Properties:
- REPOSITORY: Repository
- TAG:        Tag
- NAME:       Name of the image, --names for skip parsing as repository and tag.
- IMAGE ID:   OCI Digest. Usually different from Docker image ID. Shared for multi-platform images.
- CREATED:    Created time
- PLATFORM:   Platform
- SIZE:       Size of the unpacked snapshots
- BLOB SIZE:  Size of the blobs (such as layer tarballs) in the content store
`
	var sortBy string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: shortHelp,
		Long:  longHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := processListCmdFlags(cmd)
			if err != nil {
				return err
			}
			r, err := runtime.NewRuntime(cmd.Context(), opts.RootOptions)
			if err != nil {
				return err
			}
			defer r.Close()
			return r.ListImages(r.Context(), opts)
		},
	}
	cmd.Flags().BoolP("quiet", "q", false, "Only show numeric IDs")
	cmd.Flags().BoolP("no-trunc", "", false, "Don't truncate output")
	cmd.Flags().StringP("format", "", "", "Pretty-print images using a Go template")
	cmd.Flags().BoolP("names", "", false, "Show image names without parsing as repository and tag")
	cmd.Flags().BoolP("digests", "", false, "Show image digests")
	cmd.Flags().BoolP("all", "a", true, "(unimplemented yet, always true)")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort output by 'created' or 'size' (desc)")
	cmd.Flags().StringSliceP("filter", "f", []string{}, "Filter output based on conditions provided")
	cmd.RegisterFlagCompletionFunc("sort", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"created", "size"}, cobra.ShellCompDirectiveNoFileComp
	})
	cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "raw", "json"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func processListCmdFlags(cmd *cobra.Command) (types.ImageListOptions, error) {
	root, err := processRootCmdFlags(cmd)
	if err != nil {
		return types.ImageListOptions{}, err
	}
	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return types.ImageListOptions{}, err
	}
	noTrunc, err := cmd.Flags().GetBool("no-trunc")
	if err != nil {
		return types.ImageListOptions{}, err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return types.ImageListOptions{}, err
	}
	names, err := cmd.Flags().GetBool("names")
	if err != nil {
		return types.ImageListOptions{}, err
	}
	digests, err := cmd.Flags().GetBool("digests")
	if err != nil {
		return types.ImageListOptions{}, err
	}
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return types.ImageListOptions{}, err
	}
	sortBy, err := cmd.Flags().GetString("sort")
	if err != nil {
		return types.ImageListOptions{}, err
	}
	filters, err := cmd.Flags().GetStringSlice("filter")
	if err != nil {
		return types.ImageListOptions{}, err
	}
	return types.ImageListOptions{
		RootOptions: root,
		Stdout:      cmd.OutOrStdout(),
		Quiet:       quiet,
		NoTrunc:     noTrunc,
		Format:      format,
		Filters:     filters,
		Digests:     digests,
		Names:       names,
		All:         all,
		SortBy:      sortBy,
	}, nil
}
