package cmd

import (
	"github.com/lingdie/image-manip-server/pkg/options"
	"github.com/lingdie/image-manip-server/pkg/runtime"
	"github.com/spf13/cobra"
)

func NewCmdSearchHistory() *cobra.Command {
	searchHistoryCmd := &cobra.Command{
		Use:   "search-history IMAGE KEYWORD",
		Short: "Search image history",
		Args:  cobra.ExactArgs(2),
		RunE:  searchHistoryAction,
	}
	return searchHistoryCmd
}

func searchHistoryAction(cmd *cobra.Command, args []string) error {
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

func processSearchHistoryCmdFlags(cmd *cobra.Command) (options.SearchHistoryOptions, error) {
	var err error
	o := options.SearchHistoryOptions{}
	o.RootOptions, err = processRootCmdFlags(cmd)
	if err != nil {
		// handle error
		return o, err
	}
	return o, nil
}
