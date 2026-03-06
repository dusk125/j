package cmd

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart NAME",
	Short: "Restart a job with the same command",
	Args:              cobra.ExactArgs(1),
	RunE:              runRestart,
	ValidArgsFunction: completeJobNames(false),
}

func runRestart(cmd *cobra.Command, args []string) error {
	name := args[0]

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	// If still running, stop it first
	if meta.Status == "running" {
		job.RefreshStatus(meta)
	}
	if meta.Status == "running" {
		proc, err := os.FindProcess(meta.PID)
		if err != nil {
			return fmt.Errorf("finding process: %w", err)
		}
		proc.Signal(syscall.SIGINT)
		fmt.Printf("Stopping job %q...\n", name)

		// Wait up to 5 seconds for graceful exit
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			meta, _ = job.ReadMeta(job.MetaPath(name))
			if meta.Status != "running" {
				break
			}
			job.RefreshStatus(meta)
			if meta.Status != "running" {
				break
			}
		}

		// Force kill if still running
		if meta.Status == "running" {
			proc.Signal(syscall.SIGKILL)
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Save config before removing
	command := meta.Command
	dir := meta.Dir
	autoRemove := meta.AutoRemove

	if err := job.RemoveJob(name); err != nil {
		return fmt.Errorf("removing old job: %w", err)
	}

	newName, _, err := startJob(name, dir, autoRemove, command)
	if err != nil {
		return err
	}

	fmt.Printf("Restarted job %q\n", newName)
	return nil
}
