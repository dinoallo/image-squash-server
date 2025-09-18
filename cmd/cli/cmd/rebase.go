package cmd

import (
	"fmt"

	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

const (
	DefaultAutoSquash = false
)

func NewCmdRebase() *cobra.Command {

	var rebaseCmd = &cobra.Command{
		Use:   "rebase IMAGE_REF BASE_LAYER_DIGEST",
		Short: "Rebase a container image",
		Args:  cobra.ExactArgs(2),
		RunE:  rebaseAction,
	}
	rebaseCmd.Flags().String("new-image-name", "", "new image name, if not specified, will be the same as the original image")
	rebaseCmd.Flags().Bool("auto-squash", DefaultAutoSquash, "squash all new application layers into one")
	rebaseCmd.Flags().String("new-base-image-ref", "", "new base image ref, if not specified, the image will be rebased back")

	return rebaseCmd
}

func rebaseAction(cmd *cobra.Command, args []string) error {

	var (
		imageRef        string
		baseLayerDigest string
	)

	rebaseOptions, err := processRebaseCmdFlags(cmd)
	if err != nil {
		return err
	}
	// Positional arguments: imageRef, baseLayerDigest
	imageRef = args[0]
	baseLayerDigest = args[1]
	rebaseOptions.ImageRef = imageRef
	rebaseOptions.BaseLayerDigest = baseLayerDigest
	// init the runtime
	runtimeObj, err := runtime.NewRuntime(
		cmd.Context(),
		rebaseOptions.RootOptions,
	)
	defer func() {
		err := runtimeObj.Close()
		if err != nil {
			fmt.Printf("failed to close runtime: %v\n", err)
		}
	}()
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
	o.NewImageName, err = cmd.Flags().GetString("new-image-name")
	if err != nil {
		// handle error
		return o, err
	}
	o.AutoSquash, err = cmd.Flags().GetBool("auto-squash")
	if err != nil {
		// handle error
		return o, err
	}
	o.NewBaseImageRef, err = cmd.Flags().GetString("new-base-image-ref")
	if err != nil {
		// handle error
		return o, err
	}
	return o, nil
}
