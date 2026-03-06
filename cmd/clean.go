package cmd

import (
	"fmt"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all non-running jobs",
	RunE:  runClean,
}

func runClean(cmd *cobra.Command, args []string) error {
	jobs, err := job.ListJobs()
	if err != nil {
		return err
	}

	var removed int
	for _, m := range jobs {
		if m.Status == "running" || m.IsService() {
			continue
		}
		if err := job.RemoveJob(m.Name); err != nil {
			fmt.Printf("Failed to remove %q: %v\n", m.Name, err)
			continue
		}
		fmt.Printf("Removed %q\n", m.Name)
		removed++
	}

	if removed == 0 {
		fmt.Println("Nothing to clean.")
	}
	return nil
}
