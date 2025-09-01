package cmd

import (
	"context"

	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/nerdctl/pkg/clientutil"
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdVerifyBase() *cobra.Command {
	var (
		namespace         string
		containerdAddress string
		originalImageRef  string
		baseImageRef      string
	)
	var verifyBaseCmd = &cobra.Command{
		Use:   "verify-base IMAGE BASE_IMAGE",
		Short: "Verify if an image is based on a specific image",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			originalImageRef = args[0]
			baseImageRef = args[1]

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
			ctx := namespaces.WithNamespace(context.TODO(), namespace)
			err = runtimeObj.Verifybase(ctx, options.VerifyBaseOption{
				OriginalImage: originalImageRef,
				BaseImage:     baseImageRef,
			})
			if err != nil {
				return err
			}
			return nil
		},
	}
	verifyBaseCmd.Flags().StringVar(&containerdAddress, "containerd-address", "unix:///var/run/containerd/containerd.sock", "containerd address")
	verifyBaseCmd.Flags().StringVar(&namespace, "namespace", "k8s.io", "containerd namespace")
	// Positional arguments: originalImageRef, baseImageRef
	return verifyBaseCmd
}
