package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdTag() *cobra.Command {
	var tagCmd = &cobra.Command{
		Use:   "tag SOURCE_IMAGE TARGET_IMAGE",
		Short: "Create a new tag for an existing image",
		Args:  cobra.ExactArgs(2),
		RunE:  tagAction,
	}
	return tagCmd
}

func tagAction(cmd *cobra.Command, args []string) error {
	opts, err := processTagCmdFlags(cmd)
	if err != nil {
		return err
	}
	opts.SourceImage = args[0]
	opts.TargetImage = args[1]

	r, err := runtime.NewRuntime(cmd.Context(), opts.RootOptions)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := r.Tag(r.Context(), opts.SourceImage, opts.TargetImage); err != nil {
		return err
	}
	return nil
}

func processTagCmdFlags(cmd *cobra.Command) (options.TagOptions, error) {
	root, err := processRootCmdFlags(cmd)
	if err != nil {
		return options.TagOptions{}, err
	}
	return options.TagOptions{RootOptions: root}, nil
}
