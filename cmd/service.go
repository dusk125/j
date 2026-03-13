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

func runManage(cmd *cobra.Command, args []string) error {
	meta, err := job.Manage(args[0], manageName)
	if err != nil {
		return err
	}
	fmt.Printf("Managing %s as %q (status: %s)\n", meta.ServiceUnit, meta.Name, meta.Status)
	return nil
}

func runUnmanage(cmd *cobra.Command, args []string) error {
	unit, err := job.Unmanage(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("Stopped managing %q (%s)\n", args[0], unit)
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
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
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
