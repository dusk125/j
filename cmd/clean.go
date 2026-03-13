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
	removed, err := job.Clean()
	if err != nil {
		return err
	}

	for _, name := range removed {
		fmt.Printf("Removed %q\n", name)
	}

	if len(removed) == 0 {
		fmt.Println("Nothing to clean.")
	}
	return nil
}
