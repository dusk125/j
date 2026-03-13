package cmd

import (
	"fmt"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:               "rename OLD NEW",
	Short:             "Rename a job",
	Args:              cobra.ExactArgs(2),
	RunE:              runRename,
	ValidArgsFunction: completeJobNames(false),
}

func runRename(cmd *cobra.Command, args []string) error {
	if err := job.Rename(args[0], args[1]); err != nil {
		return err
	}
	fmt.Printf("Renamed job %q to %q\n", args[0], args[1])
	return nil
}
