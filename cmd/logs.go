package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:               "logs NAME",
	Short:             "View job logs",
	Args:              cobra.ExactArgs(1),
	RunE:              runLogs,
	ValidArgsFunction: completeJobNames(false),
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

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	if meta.IsService() {
		return serviceJournalLogs(meta)
	}

	showStdout := !logsStderr
	showStderr := !logsStdout

	// If both --stdout and --stderr are set, show both
	if logsStdout && logsStderr {
		showStdout = true
		showStderr = true
	}

	showPrefix := showStdout && showStderr

	if logsFollow && meta.Status == job.Running {
		ctx, _ := signal.NotifyContext(cmd.Context(), os.Interrupt)
		job.FollowLogs(ctx, os.Stdout, name, showStdout, showStderr, showPrefix)
		return nil
	}

	return printExisting(name, showStdout, showStderr, showPrefix)
}

func serviceJournalLogs(meta *job.Meta) error {
	jArgs := []string{"--user", "-u", meta.ServiceUnit, "--no-pager"}
	if logsTail > 0 {
		jArgs = append(jArgs, "-n", fmt.Sprintf("%d", logsTail))
	}
	if logsFollow {
		jArgs = append(jArgs, "-f")
	}
	c := exec.Command("journalctl", jArgs...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitCodeError{code: exitErr.ExitCode()}
		}
		return err
	}
	return nil
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
