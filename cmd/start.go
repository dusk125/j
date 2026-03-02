package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [flags] -- CMD [ARGS...]",
	Short: "Start a background job (detached)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runStart,
}

var startName string
var startDir string
var startAutoRm bool

func init() {
	startCmd.Flags().StringVar(&startName, "name", "", "Job name (auto-generated if empty)")
	startCmd.Flags().StringVar(&startDir, "dir", "", "Working directory (default: current directory)")
	startCmd.Flags().BoolVar(&startAutoRm, "rm", false, "Remove job after it exits")
}

func runStart(cmd *cobra.Command, args []string) error {
	name, _, err := startJob(startName, startDir, startAutoRm, args)
	if err != nil {
		return err
	}
	fmt.Printf("Started job %q\n", name)
	return nil
}

// startJob is the shared logic for launching a supervised job.
// Returns the resolved job name and metadata.
func startJob(name, dir string, autoRemove bool, args []string) (string, *job.Meta, error) {
	if err := job.EnsureJobsDir(); err != nil {
		return "", nil, fmt.Errorf("creating state directory: %w", err)
	}

	if name == "" {
		name = job.GenerateName()
	}

	if job.JobExists(name) {
		return "", nil, fmt.Errorf("job %q already exists", name)
	}

	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", nil, fmt.Errorf("getting working directory: %w", err)
		}
	}

	if err := job.CreateJobDir(name); err != nil {
		return "", nil, fmt.Errorf("creating job directory: %w", err)
	}

	meta := &job.Meta{
		Name:       name,
		Command:    args,
		Dir:        dir,
		Status:     "running",
		AutoRemove: autoRemove,
	}
	if err := job.WriteMeta(job.MetaPath(name), meta); err != nil {
		return "", nil, fmt.Errorf("writing metadata: %w", err)
	}

	self, err := os.Executable()
	if err != nil {
		return "", nil, fmt.Errorf("finding executable: %w", err)
	}

	supervisor := exec.Command(self, "_supervisor", name)
	supervisor.Dir = dir
	supervisor.Stdin = nil
	supervisor.Stdout = nil
	supervisor.Stderr = nil
	supervisor.SysProcAttr = sysProcAttr()

	if err := supervisor.Start(); err != nil {
		job.RemoveJob(name)
		return "", nil, fmt.Errorf("starting supervisor: %w", err)
	}

	job.WriteSupervisorPID(name, supervisor.Process.Pid)

	// Re-read meta to get PID set by supervisor (with a brief wait)
	return name, meta, nil
}
