package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"
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

	writeStr("Attached to job " + name + ". Ctrl+Q: detach | Ctrl+C: interrupt (x3 to kill)\r\n")

	done := make(chan struct{})

	// Follow logs in a goroutine
	go attachFollow(os.Stdout, name, done)

	// Read stdin and forward to FIFO
	ctrlCCount := 0
	var lastCtrlC time.Time
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}

		written := 0
		for i := 0; i < n; i++ {
			switch buf[i] {
			case 0x11: // Ctrl+Q — detach
				if i > written {
					fifo.Write(buf[written:i])
				}
				close(done)
				writeStr("\r\nDetached from job " + name + ".\r\n")
				return nil

			case 0x03: // Ctrl+C — interrupt / kill
				// Flush any bytes before this Ctrl+C
				if i > written {
					fifo.Write(buf[written:i])
				}
				written = i + 1

				// Reset counter if more than 2 seconds since last Ctrl+C
				if time.Since(lastCtrlC) > 2*time.Second {
					ctrlCCount = 0
				}
				ctrlCCount++
				lastCtrlC = time.Now()

				if ctrlCCount >= 3 {
					proc, err := os.FindProcess(meta.PID)
					if err == nil {
						proc.Signal(syscall.SIGKILL)
					}
					close(done)
					writeStr("\r\nKilled job " + name + ".\r\n")
					return nil
				}

				// Send SIGINT to the child process
				proc, err := os.FindProcess(meta.PID)
				if err == nil {
					proc.Signal(syscall.SIGINT)
				}

				remaining := 3 - ctrlCCount
				writeStr(fmt.Sprintf("\r\nInterrupted. %d more Ctrl+C to kill.\r\n", remaining))
			}
		}
		// Write any remaining bytes after the last special key
		if written < n {
			fifo.Write(buf[written:n])
		}
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
