package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:               "restart NAME",
	Short:             "Restart a job with the same command",
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

	if meta.IsService() {
		out, err := exec.Command("systemctl", "--user", "restart", meta.ServiceUnit).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl restart %s: %s", meta.ServiceUnit, strings.TrimSpace(string(out)))
		}
		fmt.Printf("Restarted service %q (%s)\n", name, meta.ServiceUnit)
		return nil
	}

	if err := stopJob(name); err != nil {
		return err
	}

	command := meta.Command
	dir := meta.Dir
	env := meta.Env
	autoRemove := meta.AutoRemove

	if err := job.RemoveJob(name); err != nil {
		return fmt.Errorf("removing old job: %w", err)
	}

	newName, _, err := startJob(name, dir, autoRemove, env, command)
	if err != nil {
		return err
	}

	fmt.Printf("Restarted job %q\n", newName)
	return nil
}
