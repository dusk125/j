package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var manageCmd = &cobra.Command{
	Use:               "manage UNIT",
	Short:             "Manage a user systemctl service as a j job",
	Args:              cobra.ExactArgs(1),
	RunE:              runManage,
	ValidArgsFunction: completeServiceUnits,
}

var manageName string

var unmanageCmd = &cobra.Command{
	Use:               "unmanage NAME",
	Short:             "Stop managing a systemctl service",
	Args:              cobra.ExactArgs(1),
	RunE:              runUnmanage,
	ValidArgsFunction: completeManagedServiceNames,
}

func init() {
	manageCmd.Flags().StringVar(&manageName, "name", "", "Job name (default: unit name without .service)")
}

func unitName(name string) string {
	if !strings.HasSuffix(name, ".service") {
		return name + ".service"
	}
	return name
}

func runManage(cmd *cobra.Command, args []string) error {
	unit := unitName(args[0])

	// Verify the unit exists
	if err := exec.Command("systemctl", "--user", "cat", unit).Run(); err != nil {
		return fmt.Errorf("service %q not found (systemctl --user cat %s failed)", unit, unit)
	}

	name := manageName
	if name == "" {
		name = strings.TrimSuffix(unit, ".service")
	}

	if job.JobExists(name) {
		return fmt.Errorf("job %q already exists", name)
	}

	if err := job.EnsureJobsDir(); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	if err := job.CreateJobDir(name); err != nil {
		return fmt.Errorf("creating job directory: %w", err)
	}

	meta := &job.Meta{
		Name:        name,
		Command:     []string{unit},
		ServiceUnit: unit,
		Status:      job.Running,
	}
	job.RefreshStatus(meta)

	if err := job.WriteMeta(job.MetaPath(name), meta); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	fmt.Printf("Managing %s as %q (status: %s)\n", unit, name, meta.Status)
	return nil
}

func runUnmanage(cmd *cobra.Command, args []string) error {
	name := args[0]

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}
	if !meta.IsService() {
		return fmt.Errorf("job %q is not a managed service", name)
	}

	if err := job.RemoveJob(name); err != nil {
		return fmt.Errorf("removing job: %w", err)
	}

	fmt.Printf("Stopped managing %q (%s)\n", name, meta.ServiceUnit)
	return nil
}

func completeManagedServiceNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	jobs, err := job.ListJobs()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, m := range jobs {
		if m.IsService() {
			names = append(names, m.Name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeServiceUnits(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	out, err := exec.Command("systemctl", "--user", "list-units", "--type=service", "--all", "--no-pager", "--plain", "--no-legend").Output()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var units []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			name := strings.TrimSuffix(fields[0], ".service")
			units = append(units, name)
		}
	}
	return units, cobra.ShellCompDirectiveNoFileComp
}
