package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdRemote() *cobra.Command {
	var remoteCmd = &cobra.Command{
		Use:   "remote",
		Short: "Interact with remote image registries",
	}
	remoteCmd.AddCommand(newCmdListTags())
	return remoteCmd
}

func newCmdListTags() *cobra.Command {
	var listTagsCmd = &cobra.Command{
		Use:   "list-tags REPO",
		Short: "List tags of a repository from a remote registry",
		Args:  cobra.ExactArgs(1),
		RunE:  listTagsAction,
	}
	return listTagsCmd
}

func listTagsAction(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}
	repo := args[0]
	rootOptions, err := processRootCmdFlags(cmd)
	if err != nil {
		return err
	}
	runtimeObj, err := runtime.NewRuntime(cmd.Context(), rootOptions)
	if err != nil {
		return err
	}
	defer runtimeObj.Close()

	tags, err := runtime.ListTags(runtimeObj.Context(), repo)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		runtimeObj.Println(tag)
	}
	return nil
}
