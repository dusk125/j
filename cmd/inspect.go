package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:               "inspect NAME",
	Short:             "Show detailed job metadata",
	Args:              cobra.ExactArgs(1),
	RunE:              runInspect,
	ValidArgsFunction: completeJobNames(false),
}

func runInspect(cmd *cobra.Command, args []string) error {
	meta, err := job.Inspect(args[0])
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, string(data))
	return nil
}
