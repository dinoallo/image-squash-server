package cmd

import (
	"context"

	"github.com/containerd/nerdctl/pkg/clientutil"
	"github.com/lingdie/image-rebase-server/pkg/options"
	"github.com/lingdie/image-rebase-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdSquash() *cobra.Command {
	var (
		namespace         string
		containerdAddress string
		sourceImageRef    string
		targetImageRef    string
		squashLayerCount  int
	)
	var squashCmd = &cobra.Command{
		Use:   "squash",
		Short: "Squash layers in a container image",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceImageRef = args[0]
			targetImageRef = args[1]

			containerdClient, _, _, err := clientutil.NewClient(
				context.TODO(),
				namespace,
				containerdAddress,
			)
			if err != nil {
				return err
			}

			runtimeObj, err := runtime.NewRuntime(
				containerdClient,
				namespace,
			)
			if err != nil {
				return err
			}

			err = runtimeObj.Squash(context.TODO(), options.SquashOption{
				SourceImage:      sourceImageRef,
				TargetImage:      targetImageRef,
				SquashLayerCount: squashLayerCount,
			})
			if err != nil {
				return err
			}
			return nil
		},
	}
	squashCmd.Flags().StringVar(&containerdAddress, "containerd-address", "unix:///var/run/containerd/containerd.sock", "containerd address")
	squashCmd.Flags().StringVar(&namespace, "namespace", "k8s.io", "containerd namespace")
	// Positional arguments: sourceImageRef, targetImageRef
	squashCmd.Flags().IntVar(&squashLayerCount, "squash-layer-count", 2, "squash layer count")
	return squashCmd
}
