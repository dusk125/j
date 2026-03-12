package cmd

import (
	"fmt"
	"os"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:               "rm NAME",
	Short:             "Remove a job",
	Args:              cobra.ExactArgs(1),
	RunE:              runRm,
	ValidArgsFunction: completeJobNames(false),
}

var rmForce bool

func init() {
	rmCmd.Flags().BoolVar(&rmForce, "force", false, "Force remove (even if running)")
}

func runRm(cmd *cobra.Command, args []string) error {
	name := args[0]

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	if meta.IsService() {
		if err := job.RemoveJob(name); err != nil {
			return fmt.Errorf("removing job: %w", err)
		}
		fmt.Printf("Removed managed service %q (service %s is unaffected)\n", name, meta.ServiceUnit)
		return nil
	}

	if meta.Status == job.Running && !rmForce {
		return fmt.Errorf("job %q is still running (use --force to remove)", name)
	}

	if meta.Status == job.Running && rmForce {
		job.RefreshStatus(meta)
		if meta.Status == job.Running {
			if proc, err := os.FindProcess(meta.PID); err == nil {
				proc.Kill()
			}
		}
	}

	if err := job.RemoveJob(name); err != nil {
		return fmt.Errorf("removing job: %w", err)
	}

	fmt.Printf("Removed job %q\n", name)
	return nil
}
