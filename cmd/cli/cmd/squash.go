package cmd

import (
	"fmt"

	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

const defaultDockerfileComment = "buildkit.dockerfile.v0"

func NewCmdSquash() *cobra.Command {

	var squashCmd = &cobra.Command{
		Use:   "squash IMAGE_REF",
		Short: "Squash a container image",
		Args:  cobra.ExactArgs(1),
		RunE:  squashAction,
	}
	squashCmd.Flags().String("base-layer-digest", "", "base image digest, if not specified, the image will be squashed intelligently")

	return squashCmd
}

func squashAction(cmd *cobra.Command, args []string) error {

	var (
		imageRef string
	)

	rebaseOptions, err := processSquashCmdFlags(cmd)
	if err != nil {
		return err
	}
	// Positional arguments: imageRef
	imageRef = args[0]
	rebaseOptions.ImageRef = imageRef
	rebaseOptions.AutoSquash = true

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
	// if base layer digest is not provided, we will try to detect it intelligently
	// by looking for the last layer with the default dockerfile comment
	if rebaseOptions.BaseLayerDigest == "" {
		fmt.Println("No base layer digest provided, attempting to detect intelligently...")
		// detect the base layer if not provided

		baseLayerDigest, err := runtimeObj.LastestCommentContains(runtimeObj.Context(), imageRef, defaultDockerfileComment)
		if err != nil {
			return err
		}
		rebaseOptions.BaseLayerDigest = baseLayerDigest.String()
		fmt.Printf("Detected base layer digest: %s\n", rebaseOptions.BaseLayerDigest)
	}

	// do the rebase
	if err := runtimeObj.Rebase(runtimeObj.Context(), rebaseOptions); err != nil {
		return err
	}
	return nil
}

func processSquashCmdFlags(cmd *cobra.Command) (options.RebaseOptions, error) {
	o := options.RebaseOptions{}
	var err error
	o.RootOptions, err = processRootCmdFlags(cmd)
	if err != nil {
		// handle error
		return o, err
	}
	o.BaseLayerDigest, err = cmd.Flags().GetString("base-layer-digest")
	if err != nil {
		// handle error
		return o, err
	}
	return o, nil
}
