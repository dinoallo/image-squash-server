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
	var verifyBaseCmd = &cobra.Command{
		Use:   "verify-base IMAGE BASE_IMAGE",
		Short: "Verify if an image is based on a specific image",
		Args:  cobra.MinimumNArgs(2),
		RunE:  verifyBaseAction,
	}
	return verifyBaseCmd
}

func verifyBaseAction(cmd *cobra.Command, args []string) error {
	var (
		originalImageRef = args[0]
		baseImageRef     = args[1]
	)
	verifyBaseOptions := processVerifyBaseCmdFlags(cmd)

	verifyBaseOptions.OriginalImage = originalImageRef
	verifyBaseOptions.BaseImage = baseImageRef

	containerdClient, _, _, err := clientutil.NewClient(
		context.TODO(),
		verifyBaseOptions.RootOptions.Namespace,
		verifyBaseOptions.RootOptions.ContainerdAddress,
	)
	if err != nil {
		return err
	}
	runtimeObj, err := runtime.NewRuntime(containerdClient)
	if err != nil {
		return err
	}
	ctx := namespaces.WithNamespace(context.TODO(), verifyBaseOptions.RootOptions.Namespace)
	if err := runtimeObj.Verifybase(ctx, verifyBaseOptions); err != nil {
		return err
	}
	return nil
}

func processVerifyBaseCmdFlags(cmd *cobra.Command) options.VerifyBaseOptions {
	root := processRootCmdFlags(cmd)
	return options.VerifyBaseOptions{
		RootOptions: root,
	}
}
