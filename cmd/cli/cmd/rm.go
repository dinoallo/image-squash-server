package cmd

import (
	"context"

	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/nerdctl/pkg/clientutil"
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
	return removeCmd
}

func removeAction(cmd *cobra.Command, args []string) error {
	var (
		file             = args[0]
		originalImageRef = args[1]
	)

	opts := processRemoveCmdFlags(cmd)
	opts.File = file
	opts.OriginalImage = originalImageRef
	if opts.NewImage == "" {
		opts.NewImage = opts.OriginalImage
	}
	containerdClient, _, _, err := clientutil.NewClient(
		context.TODO(),
		opts.Namespace,
		opts.ContainerdAddress,
	)
	if err != nil {
		return err
	}
	runtimeObj, err := runtime.NewRuntime(containerdClient)
	if err != nil {
		return err
	}
	ctx := namespaces.WithNamespace(context.TODO(), opts.Namespace)
	if err := runtimeObj.Remove(ctx, opts); err != nil {
		return err
	}
	return nil
}

func processRemoveCmdFlags(cmd *cobra.Command) options.RemoveOptions {
	var newImageRef string
	cmd.Flags().StringVar(&newImageRef, "new-image", "", "new image ref, if not specified, will be the same as the original image")
	root := processRootCmdFlags(cmd)
	return options.RemoveOptions{RootOptions: root, NewImage: newImageRef}
}
