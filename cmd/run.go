package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [flags] -- CMD [ARGS...]",
	Short: "Start a background job",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRun,
}

var runName string
var runDir string

func init() {
	runCmd.Flags().StringVar(&runName, "name", "", "Job name (auto-generated if empty)")
	runCmd.Flags().StringVar(&runDir, "dir", "", "Working directory (default: current directory)")
}

func runRun(cmd *cobra.Command, args []string) error {
	if err := job.EnsureJobsDir(); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	name := runName
	if name == "" {
		name = job.GenerateName()
	}

	if job.JobExists(name) {
		return fmt.Errorf("job %q already exists", name)
	}

	dir := runDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
	}

	if err := job.CreateJobDir(name); err != nil {
		return fmt.Errorf("creating job directory: %w", err)
	}

	meta := &job.Meta{
		Name:    name,
		Command: args,
		Dir:     dir,
		Status:  "running",
	}
	if err := job.WriteMeta(job.MetaPath(name), meta); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	// Find our own executable to spawn the supervisor
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	supervisor := exec.Command(self, "_supervisor", name)
	supervisor.Dir = dir
	supervisor.Stdin = nil
	supervisor.Stdout = nil
	supervisor.Stderr = nil
	supervisor.SysProcAttr = sysProcAttr()

	if err := supervisor.Start(); err != nil {
		job.RemoveJob(name)
		return fmt.Errorf("starting supervisor: %w", err)
	}

	job.WriteSupervisorPID(name, supervisor.Process.Pid)

	fmt.Printf("Started job %q (supervisor pid %d)\n", name, supervisor.Process.Pid)
	return nil
}
