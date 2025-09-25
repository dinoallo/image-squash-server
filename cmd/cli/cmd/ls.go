package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdLs() *cobra.Command {
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
			return r.ListImages(r.Context())
		},
	}
	return cmd
}
