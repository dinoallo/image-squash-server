package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdRemote() *cobra.Command {
	var remoteCmd = &cobra.Command{
		Use:   "remote",
		Short: "Interact with remote image registries",
	}
	remoteCmd.AddCommand(newCmdListTags())
	remoteCmd.Flags().Bool("insecure", false, "Allow insecure connections to registries (HTTP)")
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
	remoteOptions, err := processRemoteCmdFlags(cmd)
	if err != nil {
		return err
	}
	runtimeObj, err := runtime.NewRuntime(cmd.Context(), remoteOptions.RootOptions)
	if err != nil {
		return err
	}
	defer runtimeObj.Close()

	tags, err := runtime.ListTags(runtimeObj.Context(), repo, remoteOptions.Insecure)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		runtimeObj.Println(tag)
	}
	return nil
}

func processRemoteCmdFlags(cmd *cobra.Command) (options.RemoteOptions, error) {
	var err error
	o := options.RemoteOptions{}
	o.RootOptions, err = processRootCmdFlags(cmd)
	if err != nil {
		return o, err
	}
	o.Insecure, err = cmd.Flags().GetBool("insecure")
	if err != nil {
		return o, err
	}
	return o, nil
}
