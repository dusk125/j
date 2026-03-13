package cmd

import (
	"fmt"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:               "stop NAME",
	Short:             "Send SIGINT to a job",
	Args:              cobra.ExactArgs(1),
	RunE:              runStop,
	ValidArgsFunction: completeJobNames(true),
}

func runStop(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := job.Stop(name); err != nil {
		return err
	}
	fmt.Printf("Stopped job %q\n", name)
	return nil
}
