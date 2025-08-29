package cmd

import (
	"context"

	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/nerdctl/pkg/clientutil"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdRebase() *cobra.Command {
	var (
		namespace         string
		containerdAddress string
		originalImageRef  string
		newBaseImageRef   string
		baseImageRef      string
		newImageRef       string
	)
	var rebaseCmd = &cobra.Command{
		Use:   "rebase ORIGINAL_IMAGE NEW_BASE_IMAGE",
		Short: "Rebase a container image",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			originalImageRef = args[0]
			newBaseImageRef = args[1]

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
			if newBaseImageRef == "" {
				newBaseImageRef = originalImageRef
			}
			if newImageRef == "" {
				newImageRef = originalImageRef
			}
			ctx := namespaces.WithNamespace(context.TODO(), namespace)
			err = runtimeObj.Rebase(ctx, options.RebaseOption{
				OriginalImage: originalImageRef,
				BaseImage:     baseImageRef,
				NewBaseImage:  newBaseImageRef,
				NewImage:      newImageRef,
			})
			if err != nil {
				return err
			}
			return nil
		},
	}
	rebaseCmd.Flags().StringVar(&containerdAddress, "containerd-address", "unix:///var/run/containerd/containerd.sock", "containerd address")
	rebaseCmd.Flags().StringVar(&namespace, "namespace", "k8s.io", "containerd namespace")
	// Positional arguments: originalImageRef, newBaseImageRef
	rebaseCmd.Flags().StringVar(&baseImageRef, "base-image", "", "old base image ref, if not specified, will be the same as the original image")
	rebaseCmd.Flags().StringVar(&newImageRef, "new-image", "", "new image ref, if not specified, will be the same as the original image")
	return rebaseCmd
}
