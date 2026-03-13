package cmd

import (
	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var supervisorCmd = &cobra.Command{
	Use:    "_supervisor NAME",
	Short:  "Internal: supervise a job process",
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return job.RunSupervisor(args[0])
	},
}
