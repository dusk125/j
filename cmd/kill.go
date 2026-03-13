package cmd

import (
	"fmt"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:               "kill NAME",
	Short:             "Send SIGKILL to a job",
	Args:              cobra.ExactArgs(1),
	RunE:              runKill,
	ValidArgsFunction: completeJobNames(true),
}

func runKill(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := job.Kill(name); err != nil {
		return err
	}
	fmt.Printf("Killed job %q\n", name)
	return nil
}
