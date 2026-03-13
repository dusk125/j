package cmd

import (
	"fmt"
	"os"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var waitCmd = &cobra.Command{
	Use:               "wait NAME",
	Short:             "Wait for a job to exit and return its exit code",
	Args:              cobra.ExactArgs(1),
	RunE:              runWait,
	ValidArgsFunction: completeJobNames(true),
}

var waitAutoRm bool

func init() {
	waitCmd.Flags().BoolVar(&waitAutoRm, "rm", false, "Remove job after it exits")
}

func runWait(cmd *cobra.Command, args []string) error {
	name := args[0]
	code, err := job.Wait(name)
	if err != nil {
		return err
	}

	if waitAutoRm {
		job.RemoveJob(name)
	}

	if code != 0 {
		fmt.Fprintf(os.Stderr, "Job %q exited with code %d\n", name, code)
	}
	return exitCodeError{code}
}
