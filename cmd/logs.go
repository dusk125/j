package cmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs NAME",
	Short: "View job logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

var (
	logsFollow bool
	logsStdout bool
	logsStderr bool
	logsTail   int
)

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().BoolVar(&logsStdout, "stdout", false, "Show only stdout")
	logsCmd.Flags().BoolVar(&logsStderr, "stderr", false, "Show only stderr")
	logsCmd.Flags().IntVar(&logsTail, "tail", 0, "Number of lines to show from the end")
}

func runLogs(cmd *cobra.Command, args []string) error {
	name := args[0]

	if !job.JobExists(name) {
		return fmt.Errorf("job %q not found", name)
	}

	showStdout := !logsStderr
	showStderr := !logsStdout

	// If both --stdout and --stderr are set, show both
	if logsStdout && logsStderr {
		showStdout = true
		showStderr = true
	}

	showPrefix := showStdout && showStderr

	if logsFollow {
		// Print existing logs first
		printExisting(name, showStdout, showStderr, showPrefix)

		stop := make(chan struct{})
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		go func() {
			<-sig
			close(stop)
		}()
		job.FollowLogs(os.Stdout, name, showStdout, showStderr, showPrefix, stop)
		return nil
	}

	return printExisting(name, showStdout, showStderr, showPrefix)
}

func printExisting(name string, showStdout, showStderr, showPrefix bool) error {
	var entries []job.LogEntry
	var err error

	if showStdout && showStderr {
		entries, err = job.MergedLogs(name)
	} else if showStdout {
		entries, err = job.ReadLogFile(job.StdoutLogPath(name), "stdout")
	} else {
		entries, err = job.ReadLogFile(job.StderrLogPath(name), "stderr")
	}
	if err != nil {
		return err
	}

	if logsTail > 0 {
		entries = job.TailEntries(entries, logsTail)
	}

	job.PrintEntries(os.Stdout, entries, showPrefix)
	return nil
}
