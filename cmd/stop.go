package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	return nil
}
