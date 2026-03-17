package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// exitCodeError is a sentinel error that carries a process exit code.
// Returning this from a RunE lets defers run before the process exits.
type exitCodeError struct {
	code int
}

func (e exitCodeError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

var rootCmd = &cobra.Command{
	Use:           "j",
	Short:         "Local process manager",
	Long:          "A Docker/Podman-inspired CLI tool for managing local background processes.",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var ec exitCodeError
		if errors.As(err, &ec) {
			os.Exit(ec.code)
		}
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(supervisorCmd)
	rootCmd.AddCommand(psCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(killCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(waitCmd)
	rootCmd.AddCommand(eventsCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(manageCmd)
	rootCmd.AddCommand(unmanageCmd)
}
