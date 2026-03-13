package cmd

import (
	"fmt"

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
	if err := job.Remove(name, rmForce); err != nil {
		return err
	}
	fmt.Printf("Removed job %q\n", name)
	return nil
}
