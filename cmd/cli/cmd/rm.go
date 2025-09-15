package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdRemove() *cobra.Command {
	var removeCmd = &cobra.Command{
		Use:   "remove FILE IMAGE",
		Short: "Remove a file from a container image",
		Args:  cobra.MinimumNArgs(2),
		RunE:  removeAction,
	}
	removeCmd.Flags().String("new-image", "", "new image ref, if not specified, will be the same as the original image")
	return removeCmd
}

func removeAction(cmd *cobra.Command, args []string) error {
	var (
		file             = args[0]
		originalImageRef = args[1]
	)

	opts, err := processRemoveCmdFlags(cmd)
	if err != nil {
		return err
	}
	opts.File = file
	opts.OriginalImage = originalImageRef
	if opts.NewImage == "" {
		opts.NewImage = opts.OriginalImage
	}
	runtimeObj, err := runtime.NewRuntime(
		cmd.Context(),
		opts.RootOptions,
	)
	if err != nil {
		return err
	}
	if err := runtimeObj.Remove(opts); err != nil {
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
	o.NewImage, err = cmd.Flags().GetString("new-image")
	if err != nil {
		// handle error
		return o, err
	}
	return o, nil
}
