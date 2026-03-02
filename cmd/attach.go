package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var attachCmd = &cobra.Command{
	Use:   "attach NAME",
	Short: "Attach to a running job's stdin/stdout/stderr",
	Args:  cobra.ExactArgs(1),
	RunE:  runAttach,
}

func runAttach(cmd *cobra.Command, args []string) error {
	name := args[0]

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	job.RefreshStatus(meta)
	if meta.Status != "running" {
		return fmt.Errorf("job %q is not running (status: %s)", name, meta.Status)
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal")
	}

	// Open the stdin FIFO for writing
	fifo, err := os.OpenFile(job.StdinPipePath(name), os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("opening stdin pipe: %w (is the job running?)", err)
	}
	defer fifo.Close()

	// Set terminal to raw mode
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("setting raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	writeStr := func(s string) {
		os.Stdout.WriteString(s)
	}

	writeStr("Attached to job " + name + ". Press Ctrl+Q to detach.\r\n")

	done := make(chan struct{})

	// Follow logs in a goroutine
	go attachFollow(os.Stdout, name, done)

	// Read stdin and forward to FIFO
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}
		// Scan for Ctrl+Q (0x11)
		for i := 0; i < n; i++ {
			if buf[i] == 0x11 {
				// Write everything before the detach key
				if i > 0 {
					fifo.Write(buf[:i])
				}
				close(done)
				writeStr("\r\nDetached from job " + name + ".\r\n")
				return nil
			}
		}
		fifo.Write(buf[:n])
	}

	close(done)
	writeStr("\r\nDetached from job " + name + ".\r\n")
	return nil
}

// attachFollow streams new log lines to the writer with \r\n line endings for raw mode.
func attachFollow(w io.Writer, name string, stop <-chan struct{}) {
	var stdoutOffset, stderrOffset int64

	for {
		select {
		case <-stop:
			return
		default:
		}

		var entries []attachEntry

		outs, off := readNewLogEntries(job.StdoutLogPath(name), stdoutOffset)
		stdoutOffset = off
		entries = append(entries, outs...)

		errs, off := readNewLogEntries(job.StderrLogPath(name), stderrOffset)
		stderrOffset = off
		entries = append(entries, errs...)

		if len(entries) > 0 {
			sort.SliceStable(entries, func(i, j int) bool {
				return entries[i].ts < entries[j].ts
			})
			for _, e := range entries {
				// Use \r\n for raw terminal mode
				fmt.Fprint(w, e.line+"\r\n")
			}
		}

		time.Sleep(50 * time.Millisecond)
	}
}

type attachEntry struct {
	ts   int64
	line string
}

func readNewLogEntries(path string, offset int64) ([]attachEntry, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset
	}
	defer f.Close()

	if offset > 0 {
		f.Seek(offset, io.SeekStart)
	}

	var entries []attachEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		var ts int64
		fmt.Sscanf(parts[0], "%d", &ts)
		entries = append(entries, attachEntry{ts: ts, line: parts[1]})
	}

	newOffset, _ := f.Seek(0, io.SeekCurrent)
	return entries, newOffset
}
