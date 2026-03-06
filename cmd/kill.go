package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:   "kill NAME",
	Short: "Send SIGKILL to a job",
	Args:              cobra.ExactArgs(1),
	RunE:              runKill,
	ValidArgsFunction: completeJobNames(true),
}

func runKill(cmd *cobra.Command, args []string) error {
	name := args[0]

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	if meta.Status != "running" {
		return fmt.Errorf("job %q is not running (status: %s)", name, meta.Status)
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}

	if err := proc.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("sending SIGKILL: %w", err)
	}

	fmt.Printf("Sent SIGKILL to job %q (pid %d)\n", name, meta.PID)
	return nil
}
