package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	job.RefreshStatus(meta)
	if meta.Status != job.Running {
		return fmt.Errorf("job %q is not running (status: %s)", name, meta.Status)
	}

	if meta.IsService() {
		out, err := exec.Command("systemctl", "--user", "kill", "--signal=SIGKILL", meta.ServiceUnit).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl kill %s: %s", meta.ServiceUnit, strings.TrimSpace(string(out)))
		}
		fmt.Printf("Sent SIGKILL to service %q (%s)\n", name, meta.ServiceUnit)
		return nil
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}

	if err := proc.Signal(os.Kill); err != nil {
		return fmt.Errorf("sending SIGKILL: %w", err)
	}

	fmt.Printf("Sent SIGKILL to job %q (pid %d)\n", name, meta.PID)
	return nil
}
