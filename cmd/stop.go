package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	job.RefreshStatus(meta)
	if meta.Status != job.Running {
		return fmt.Errorf("job %q is not running (status: %s)", name, meta.Status)
	}

	if meta.IsService() {
		out, err := exec.Command("systemctl", "--user", "stop", meta.ServiceUnit).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl stop %s: %s", meta.ServiceUnit, strings.TrimSpace(string(out)))
		}
		fmt.Printf("Stopped service %q (%s)\n", name, meta.ServiceUnit)
		return nil
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("sending SIGINT: %w", err)
	}

	fmt.Printf("Sent SIGINT to job %q (pid %d)\n", name, meta.PID)

	timeout := 5 * time.Second
	if waitForProcessExit(name, timeout) {
		fmt.Printf("Job %q exited\n", name)
		return nil
	}

	fmt.Printf("Job %q did not exit within %s, sending SIGKILL\n", name, timeout)
	if err := proc.Signal(os.Kill); err != nil {
		return fmt.Errorf("sending SIGKILL: %w", err)
	}

	waitForProcessExit(name, 0)
	fmt.Printf("Job %q killed\n", name)
	return nil
}

// waitForProcessExit polls until the job is no longer running.
// If timeout is 0, it waits indefinitely.
// Returns true if the process exited within the timeout.
func waitForProcessExit(name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		meta, err := job.ReadMeta(job.MetaPath(name))
		if err != nil {
			return true
		}
		job.RefreshStatus(meta)
		if meta.Status != job.Running {
			return true
		}
		if timeout > 0 && time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}
