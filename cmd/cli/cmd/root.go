package cmd

import (
	"github.com/spf13/cobra"
)

var Root = New()

func New() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "image-manip",
		Short: "git like image utils",
	}

	rootCmd.AddCommand(NewCmdRebase())

	return rootCmd
}
