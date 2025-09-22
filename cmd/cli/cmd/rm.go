package cmd

import (
	"fmt"

	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdRemove() *cobra.Command {
	var removeCmd = &cobra.Command{
		Use:   "remove IMAGE_REF FILE",
		Short: "Remove a file from a container image",
		Args:  cobra.ExactArgs(2),
		RunE:  removeAction,
	}
	removeCmd.Flags().String("new-image-name", "", "new image name, if not specified, will be the same as the original image")
	return removeCmd
}

func removeAction(cmd *cobra.Command, args []string) error {
	var (
		imageRef = args[0]
		file     = args[1]
	)

	opts, err := processRemoveCmdFlags(cmd)
	if err != nil {
		return err
	}
	opts.File = file
	opts.ImageRef = imageRef
	runtimeObj, err := runtime.NewRuntime(
		cmd.Context(),
		opts.RootOptions,
	)
	if err != nil {
		return err
	}
	defer func() {
		err := runtimeObj.Close()
		if err != nil {
			fmt.Printf("failed to close runtime: %v\n", err)
		}
	}()
	if err := runtimeObj.Remove(runtimeObj.Context(), opts); err != nil {
		return err
	}
	return nil
}

func processRemoveCmdFlags(cmd *cobra.Command) (options.RemoveOptions, error) {
	var err error
	o := options.RemoveOptions{}
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
	return o, nil
}
