package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

const (
	DefaultAutoSquash = false
)

func NewCmdRebase() *cobra.Command {

	var rebaseCmd = &cobra.Command{
		Use:   "rebase ORIGINAL_IMAGE NEW_BASE_IMAGE",
		Short: "Rebase a container image",
		Args:  cobra.MinimumNArgs(2),
		RunE:  rebaseAction,
	}
	_ = processRebaseCmdFlags(rebaseCmd)

	return rebaseCmd
}

func rebaseAction(cmd *cobra.Command, args []string) error {

	var (
		originalImageRef string
		newBaseImageRef  string
	)

	rebaseOptions := processRebaseCmdFlags(cmd)
	// Positional arguments: originalImageRef, newBaseImageRef
	originalImageRef = args[0]
	newBaseImageRef = args[1]
	rebaseOptions.OriginalImage = originalImageRef
	rebaseOptions.NewBaseImage = newBaseImageRef
	// base image and new image default to original image if not specified
	if rebaseOptions.BaseImage == "" {
		rebaseOptions.BaseImage = originalImageRef
	}
	if rebaseOptions.NewImage == "" {
		rebaseOptions.NewImage = originalImageRef
	}
	// init the runtime
	runtimeObj, err := runtime.NewRuntime(
		cmd.Context(),
		rebaseOptions.RootOptions,
	)
	if err != nil {
		return err
	}
	// do the rebase
	if err := runtimeObj.Rebase(rebaseOptions); err != nil {
		return err
	}
	return nil
}

func processRebaseCmdFlags(cmd *cobra.Command) options.RebaseOptions {
	var (
		baseImageRef string
		newImageRef  string
		autoSquash   bool
	)
	cmd.Flags().StringVar(&baseImageRef, "base-image", "", "old base image ref, if not specified, will be the same as the original image")
	cmd.Flags().StringVar(&newImageRef, "new-image", "", "new image ref, if not specified, will be the same as the original image")
	cmd.Flags().BoolVar(&autoSquash, "auto-squash", DefaultAutoSquash, "squash all new application layers into one (disabled by default)")
	return options.RebaseOptions{
		RootOptions: processRootCmdFlags(cmd),
		BaseImage:   baseImageRef,
		NewImage:    newImageRef,
		AutoSquash:  autoSquash,
	}
}
