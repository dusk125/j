package cmd

import (
	"fmt"
	"os"
	"time"

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

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	// Already exited
	if meta.Status != job.Running {
		if waitAutoRm {
			job.RemoveJob(name)
		}
		return exitWithMeta(meta)
	}

	// Poll until exit
	for {
		time.Sleep(100 * time.Millisecond)
		meta, err = job.ReadMeta(job.MetaPath(name))
		if err != nil {
			return fmt.Errorf("reading job state: %w", err)
		}
		job.RefreshStatus(meta)
		if meta.Status != job.Running {
			if waitAutoRm {
				job.RemoveJob(name)
			}
			return exitWithMeta(meta)
		}
	}
}

func exitWithMeta(meta *job.Meta) error {
	code := 0
	if meta.ExitCode != nil {
		code = *meta.ExitCode
	}
	if code != 0 {
		fmt.Fprintf(os.Stderr, "Job %q exited with code %d\n", meta.Name, code)
	}
	return exitCodeError{code}
}
