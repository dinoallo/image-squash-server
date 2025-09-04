package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/spf13/cobra"
)

const (
	DefaultAppName           = "image-manip"
	DefaultContainerdAddress = "unix:///var/run/containerd/containerd.sock"
	DefaultNamespace         = "k8s.io"
)

var Root = New()

func New() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "image-manip",
		Short: "git like image utils",
	}
	_ = processRootCmdFlags(rootCmd)

	rootCmd.AddCommand(NewCmdRebase())
	rootCmd.AddCommand(NewCmdRemove())
	rootCmd.AddCommand(NewCmdVerifyBase())

	return rootCmd
}

func processRootCmdFlags(cmd *cobra.Command) options.RootOptions {
	var (
		containerdAddress string
		namespace         string
	)
	cmd.PersistentFlags().StringVar(&containerdAddress, "containerd-address", DefaultContainerdAddress, "containerd address")
	cmd.PersistentFlags().StringVar(&namespace, "namespace", DefaultNamespace, "containerd namespace")
	return options.RootOptions{
		ContainerdAddress: containerdAddress,
		Namespace:         namespace,
	}
}
