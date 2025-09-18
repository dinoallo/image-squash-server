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
		Use:   "rebase ORIGINAL_IMAGE BASE_LAYER",
		Short: "Rebase a container image",
		Args:  cobra.MinimumNArgs(2),
		RunE:  rebaseAction,
	}
	rebaseCmd.Flags().String("new-image", "", "new image ref, if not specified, will be the same as the original image")
	rebaseCmd.Flags().Bool("auto-squash", DefaultAutoSquash, "squash all new application layers into one (disabled by default)")
	rebaseCmd.Flags().String("new-base-image", "", "new base image ref, if not specified, will be the same as the base layer")

	return rebaseCmd
}

func rebaseAction(cmd *cobra.Command, args []string) error {

	var (
		originalImageRef string
		baseLayerRef     string
	)

	rebaseOptions, err := processRebaseCmdFlags(cmd)
	if err != nil {
		return err
	}
	// Positional arguments: originalImageRef, baseLayerRef
	originalImageRef = args[0]
	baseLayerRef = args[1]
	rebaseOptions.OriginalImage = originalImageRef
	rebaseOptions.BaseLayer = baseLayerRef
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
	o.NewBaseImage, err = cmd.Flags().GetString("new-base-image")
	if err != nil {
		// handle error
		return o, err
	}
	return o, nil
}
