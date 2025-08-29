package cmd

import (
	"context"

	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/nerdctl/pkg/clientutil"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdRemove() *cobra.Command {
	var (
		namespace         string
		containerdAddress string
		originalImageRef  string
		newImageRef       string
	)
	var removeCmd = &cobra.Command{
		Use:   "remove FILE IMAGE",
		Short: "Remove a file from a container image",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			file := args[0]
			originalImageRef = args[1]

			//TODO: reuse runtime and containerd client
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
			if newImageRef == "" {
				newImageRef = originalImageRef
			}
			ctx := namespaces.WithNamespace(context.TODO(), namespace)
			err = runtimeObj.Remove(ctx, options.RemoveOption{
				File:          file,
				OriginalImage: originalImageRef,
				NewImage:      newImageRef,
			})
			if err != nil {
				return err
			}
			return nil
		},
	}
	removeCmd.Flags().StringVar(&containerdAddress, "containerd-address", "unix:///var/run/containerd/containerd.sock", "containerd address")
	removeCmd.Flags().StringVar(&namespace, "namespace", "k8s.io", "containerd namespace")
	// Positional arguments: originalImageRef, newBaseImageRef
	removeCmd.Flags().StringVar(&newImageRef, "new-image", "", "new image ref, if not specified, will be the same as the original image")
	return removeCmd
}
