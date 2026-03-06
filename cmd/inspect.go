package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect NAME",
	Short: "Show detailed job metadata",
	Args:              cobra.ExactArgs(1),
	RunE:              runInspect,
	ValidArgsFunction: completeJobNames(false),
}

func runInspect(cmd *cobra.Command, args []string) error {
	name := args[0]

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	job.RefreshStatus(meta)

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, string(data))
	return nil
}
