package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdHistory() *cobra.Command {
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Inspect image history",
	}
	historyCmd.AddCommand(newHistorySearchCmd())
	historyCmd.AddCommand(newHistoryListCmd())
	addHistoryFlags(historyCmd)
	return historyCmd
}

func addHistoryFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("format", "f", "", "Format the output using the given Go template, e.g, '{{json .}}'")
	cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json"}, cobra.ShellCompDirectiveNoFileComp
	})
	cmd.PersistentFlags().BoolP("quiet", "q", false, "Only show numeric IDs")
	cmd.PersistentFlags().Bool("no-trunc", false, "Don't truncate output")
}

func newHistoryListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list IMAGE",
		Short: "List image history",
		Args:  cobra.ExactArgs(1),
		RunE:  historyListAction,
	}
	return listCmd
}

func newHistorySearchCmd() *cobra.Command {
	searchCmd := &cobra.Command{
		Use:   "search IMAGE KEYWORD",
		Short: "Search image history for a Dockerfile keyword",
		Args:  cobra.ExactArgs(2),
		RunE:  historySearchAction,
	}
	return searchCmd
}

func historyListAction(cmd *cobra.Command, args []string) error {
	historyOptions, err := processHistoryCmdFlags(cmd)
	if err != nil {
		return err
	}
	historyOptions.ImageRef = args[0]
	runtimeObj, err := runtime.NewRuntime(cmd.Context(), historyOptions.RootOptions)
	if err != nil {
		return err
	}
	if err := runtimeObj.ListImageHistory(runtimeObj.Context(), historyOptions); err != nil {
		return err
	}
	return nil
}

func historySearchAction(cmd *cobra.Command, args []string) error {
	searchHistoryOptions, err := processSearchHistoryCmdFlags(cmd)
	if err != nil {
		return err
	}
	searchHistoryOptions.ImageRef = args[0]
	searchHistoryOptions.Keyword = args[1]
	runtimeObj, err := runtime.NewRuntime(cmd.Context(), searchHistoryOptions.RootOptions)
	if err != nil {
		return err
	}
	if err := runtimeObj.SearchImageHistory(runtimeObj.Context(), searchHistoryOptions); err != nil {
		return err
	}
	return nil
}

func processHistoryCmdFlags(cmd *cobra.Command) (options.HistoryOptions, error) {
	var err error
	o := options.HistoryOptions{}
	o.RootOptions, err = processRootCmdFlags(cmd)
	if err != nil {
		// handle error
		return o, err
	}
	o.Format, err = cmd.Flags().GetString("format")
	if err != nil {
		// handle error
		return o, err
	}
	o.Quiet, err = cmd.Flags().GetBool("quiet")
	if err != nil {
		// handle error
		return o, err
	}
	o.NoTrunc, err = cmd.Flags().GetBool("no-trunc")
	if err != nil {
		// handle error
		return o, err
	}
	return o, nil
}

func processSearchHistoryCmdFlags(cmd *cobra.Command) (options.SearchHistoryOptions, error) {
	var err error
	o := options.SearchHistoryOptions{}
	o.HistoryOptions, err = processHistoryCmdFlags(cmd)
	if err != nil {
		// handle error
		return o, err
	}
	return o, nil
}
