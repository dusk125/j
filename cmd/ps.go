package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List jobs",
	RunE:  runPs,
}

var psAll bool

func init() {
	psCmd.Flags().BoolVarP(&psAll, "all", "a", false, "Show all jobs (including stopped)")
}

func runPs(cmd *cobra.Command, args []string) error {
	jobs, err := job.ListJobs()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tPID\tRUNTIME\tCOMMAND")

	for _, m := range jobs {
		if !psAll && m.Status != "running" {
			continue
		}
		runtime := formatRuntime(m)
		cmdStr := strings.Join(m.Command, " ")
		if len(cmdStr) > 40 {
			cmdStr = cmdStr[:37] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", m.Name, m.Status, m.PID, runtime, cmdStr)
	}
	w.Flush()
	return nil
}

func formatRuntime(m *job.Meta) string {
	if m.StartedAt.IsZero() {
		return "-"
	}
	end := m.EndedAt
	if end.IsZero() {
		end = time.Now()
	}
	d := end.Sub(m.StartedAt).Round(time.Second)
	return d.String()
}
