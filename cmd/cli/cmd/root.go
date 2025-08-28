package cmd

import (
	"github.com/spf13/cobra"
)

var Root = New()

func New() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "image-rebase",
		Short: "git rebase like image utils",
	}

	rootCmd.AddCommand(NewCmdSquash())

	return rootCmd
}
