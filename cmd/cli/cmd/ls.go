package cmd

import (
	"fmt"
	"strings"

	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdLs() *cobra.Command {
	var sortBy string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List images",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := processRootCmdFlags(cmd)
			if err != nil {
				return err
			}
			r, err := runtime.NewRuntime(cmd.Context(), root)
			if err != nil {
				return err
			}
			defer r.Close()

			var opt runtime.ListImagesOptions
			switch strings.ToLower(sortBy) {
			case "", "none":
				opt.SortBy = runtime.SortNone
			case "created":
				opt.SortBy = runtime.SortCreated
			case "size":
				opt.SortBy = runtime.SortSize
			default:
				return fmt.Errorf("unknown sort key: %s (allowed: created, size)", sortBy)
			}
			return r.ListImages(r.Context(), opt)
		},
	}
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort output by 'created' or 'size' (desc)")
	cmd.RegisterFlagCompletionFunc("sort", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"created", "size"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}
