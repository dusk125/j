package cmd

import (
	"fmt"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:               "restart NAME",
	Short:             "Restart a job with the same command",
	Args:              cobra.ExactArgs(1),
	RunE:              runRestart,
	ValidArgsFunction: completeJobNames(false),
}

func runRestart(cmd *cobra.Command, args []string) error {
	name := args[0]
	newName, _, err := job.Restart(name, nil)
	if err != nil {
		return err
	}
	fmt.Printf("Restarted job %q\n", newName)
	return nil
}
