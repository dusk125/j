package cmd

import (
	"fmt"
	"os/exec"
	"strings"

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
var startEnv []string

func init() {
	startCmd.Flags().StringVar(&startName, "name", "", "Job name (auto-generated if empty)")
	startCmd.Flags().StringVar(&startDir, "dir", "", "Working directory (default: current directory)")
	startCmd.Flags().BoolVar(&startAutoRm, "rm", false, "Remove job after it exits")
	startCmd.Flags().StringArrayVarP(&startEnv, "env", "e", nil, "Set environment variables (KEY=VALUE)")
}

func runStart(cmd *cobra.Command, args []string) error {
	// If a single arg matches an existing managed service, start it via systemctl
	if len(args) == 1 && job.JobExists(args[0]) {
		meta, err := job.ReadMeta(job.MetaPath(args[0]))
		if err == nil && meta.IsService() {
			job.RefreshStatus(meta)
			if meta.Status == job.Running {
				return fmt.Errorf("service %q is already running", args[0])
			}
			out, err := exec.Command("systemctl", "--user", "start", meta.ServiceUnit).CombinedOutput()
			if err != nil {
				return fmt.Errorf("systemctl start %s: %s", meta.ServiceUnit, strings.TrimSpace(string(out)))
			}
			fmt.Printf("Started service %q (%s)\n", args[0], meta.ServiceUnit)
			return nil
		}
	}

	name, _, err := job.Start(args, job.StartOptions{
		Name:       startName,
		Dir:        startDir,
		Env:        startEnv,
		AutoRemove: startAutoRm,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Started job %q\n", name)
	return nil
}
