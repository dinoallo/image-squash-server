package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// GitCommit is injected at build time using -ldflags
	GitCommit string
)

func NewCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version (git commit sha)",
		Run: func(cmd *cobra.Command, args []string) {
			versionStr := fmt.Sprintf("image-manip %s", GitCommit)
			fmt.Println(versionStr)
		},
	}
}
