package cmd

import (
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
	verifyBaseOptions, err := processVerifyBaseCmdFlags(cmd)
	if err != nil {
		return err
	}

	verifyBaseOptions.OriginalImage = originalImageRef
	verifyBaseOptions.BaseImage = baseImageRef
	runtimeObj, err := runtime.NewRuntime(
		cmd.Context(),
		verifyBaseOptions.RootOptions,
	)
	if err != nil {
		return err
	}
	if err := runtimeObj.Verifybase(runtimeObj.Context(), verifyBaseOptions); err != nil {
		return err
	}
	return nil
}

func processVerifyBaseCmdFlags(cmd *cobra.Command) (options.VerifyBaseOptions, error) {
	root, err := processRootCmdFlags(cmd)
	if err != nil {
		// handle error
		return options.VerifyBaseOptions{}, err
	}
	return options.VerifyBaseOptions{
		RootOptions: root,
	}, nil
}
