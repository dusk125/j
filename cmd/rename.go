package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename OLD NEW",
	Short: "Rename a job",
	Args:  cobra.ExactArgs(2),
	RunE:  runRename,
}

func runRename(cmd *cobra.Command, args []string) error {
	oldName := args[0]
	newName := args[1]

	if oldName == newName {
		return fmt.Errorf("old and new names are the same")
	}

	meta, err := job.ReadMeta(job.MetaPath(oldName))
	if err != nil {
		return fmt.Errorf("job %q not found", oldName)
	}

	job.RefreshStatus(meta)

	if job.JobExists(newName) {
		return fmt.Errorf("job %q already exists", newName)
	}

	oldDir := job.JobDir(oldName)
	newDir := job.JobDir(newName)

	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("renaming job directory: %w", err)
	}

	meta.Name = newName
	if err := job.WriteMeta(job.MetaPath(newName), meta); err != nil {
		// Try to undo the rename
		os.Rename(newDir, oldDir)
		return fmt.Errorf("updating metadata: %w", err)
	}

	// If the job is running, the supervisor still references the old name for
	// path construction. Create a symlink so those paths continue to resolve.
	if meta.Status == "running" {
		rel, err := filepath.Rel(filepath.Dir(oldDir), newDir)
		if err != nil {
			rel = newDir
		}
		if err := os.Symlink(rel, oldDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not create compatibility symlink: %v\n", err)
		}
	}

	fmt.Printf("Renamed job %q to %q\n", oldName, newName)
	return nil
}
