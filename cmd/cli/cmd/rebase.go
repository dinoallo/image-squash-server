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
	rebaseCmd.Flags().String("base-image", "", "old base image ref, if not specified, will be the same as the original image")
	rebaseCmd.Flags().String("new-image", "", "new image ref, if not specified, will be the same as the original image")
	rebaseCmd.Flags().Bool("auto-squash", DefaultAutoSquash, "squash all new application layers into one (disabled by default)")

	return rebaseCmd
}

func rebaseAction(cmd *cobra.Command, args []string) error {

	var (
		originalImageRef string
		newBaseImageRef  string
	)

	rebaseOptions, err := processRebaseCmdFlags(cmd)
	if err != nil {
		return err
	}
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
	if err := runtimeObj.Rebase(runtimeObj.Context(), rebaseOptions); err != nil {
		return err
	}
	return nil
}

func processRebaseCmdFlags(cmd *cobra.Command) (options.RebaseOptions, error) {
	o := options.RebaseOptions{}
	var err error
	o.RootOptions, err = processRootCmdFlags(cmd)
	if err != nil {
		// handle error
		return o, err
	}
	o.BaseImage, err = cmd.Flags().GetString("base-image")
	if err != nil {
		// handle error
		return o, err
	}
	o.NewImage, err = cmd.Flags().GetString("new-image")
	if err != nil {
		// handle error
		return o, err
	}
	o.AutoSquash, err = cmd.Flags().GetBool("auto-squash")
	if err != nil {
		// handle error
		return o, err
	}
	return o, nil
}
