package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/spf13/cobra"
)

const (
	DefaultAppName           = "image-manip"
	DefaultContainerdAddress = "unix:///var/run/containerd/containerd.sock"
	DefaultNamespace         = "k8s.io"
	DefaultLogLevel          = "info"
)

var Root = New()

func New() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "image-manip",
		Short: "git like image utils",
	}
	rootCmd.PersistentFlags().String("containerd-address", DefaultContainerdAddress, "containerd address")
	rootCmd.PersistentFlags().StringP("namespace", "n", DefaultNamespace, "containerd namespace")
	rootCmd.PersistentFlags().StringP("log-level", "l", DefaultLogLevel, "log level")

	rootCmd.AddCommand(NewCmdRebase())
	rootCmd.AddCommand(NewCmdRemove())
	rootCmd.AddCommand(NewCmdVerifyBase())
	rootCmd.AddCommand(NewCmdHistory())
	rootCmd.AddCommand(NewCmdSquash())
	rootCmd.AddCommand(NewCmdVersion())
	rootCmd.AddCommand(NewCmdRemote())

	return rootCmd
}

func processRootCmdFlags(cmd *cobra.Command) (options.RootOptions, error) {
	o := options.RootOptions{}
	var err error
	o.ContainerdAddress, err = cmd.Flags().GetString("containerd-address")
	if err != nil {
		// handle error
		return o, err
	}
	o.Namespace, err = cmd.Flags().GetString("namespace")
	if err != nil {
		// handle error
		return o, err
	}
	o.LogLevel, err = cmd.Flags().GetString("log-level")
	if err != nil {
		// handle error
		return o, err
	}
	return o, nil
}
