package cmd

import (
	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

func completeJobNames(onlyRunning bool) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		jobs, err := job.ListJobs()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var names []string
		for _, m := range jobs {
			if onlyRunning && m.Status != "running" {
				continue
			}
			names = append(names, m.Name)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}
