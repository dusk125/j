package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Show jobs that recently changed state",
	RunE:  runEvents,
}

var eventsSince string

func init() {
	eventsCmd.Flags().StringVarP(&eventsSince, "since", "s", "1h", "Show events within this duration (e.g. 30m, 2h, 24h)")
}

func runEvents(cmd *cobra.Command, args []string) error {
	since, err := time.ParseDuration(eventsSince)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", eventsSince, err)
	}

	cutoff := time.Now().Add(-since)

	jobs, err := job.ListJobs()
	if err != nil {
		return err
	}

	// Collect jobs with a state change after the cutoff.
	// "State change" means: the job ended, or it started recently.
	type event struct {
		meta *job.Meta
		when time.Time
		kind job.Status
	}

	var events []event
	for _, m := range jobs {
		// Job ended after cutoff
		if !m.EndedAt.IsZero() && m.EndedAt.After(cutoff) {
			events = append(events, event{meta: m, when: m.EndedAt, kind: m.Status})
		}
		// Job started after cutoff and is still running
		if m.Status == job.Running && !m.StartedAt.IsZero() && m.StartedAt.After(cutoff) {
			events = append(events, event{meta: m, when: m.StartedAt, kind: job.Started})
		}
	}

	// Sort newest first
	sort.Slice(events, func(i, j int) bool {
		return events[i].when.After(events[j].when)
	})

	if len(events) == 0 {
		fmt.Printf("No events in the last %s.\n", since)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tEVENT\tEXIT\tAGO\tCOMMAND")

	for _, e := range events {
		ago := formatAgo(e.when)
		exitStr := "-"
		if e.meta.ExitCode != nil {
			exitStr = fmt.Sprintf("%d", *e.meta.ExitCode)
		}
		cmdStr := strings.Join(e.meta.Command, " ")
		if len(cmdStr) > 40 {
			cmdStr = cmdStr[:37] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", e.meta.Name, e.kind, exitStr, ago, cmdStr)
	}
	w.Flush()
	return nil
}

func formatAgo(t time.Time) string {
	d := time.Since(t).Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh ago", h)
		}
		return fmt.Sprintf("%dh%dm ago", h, m)
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
