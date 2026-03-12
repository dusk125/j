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
	Use:               "attach NAME",
	Short:             "Attach to a running job's stdin/stdout/stderr",
	Args:              cobra.ExactArgs(1),
	RunE:              runAttach,
	ValidArgsFunction: completeJobNames(true),
}

func runAttach(cmd *cobra.Command, args []string) error {
	name := args[0]

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	if meta.IsService() {
		return fmt.Errorf("job %q is a managed systemctl service; use 'j logs %s' instead", name, name)
	}

	job.RefreshStatus(meta)
	if meta.Status != job.Running {
		return fmt.Errorf("job %q is not running (status: %s)", name, meta.Status)
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal")
	}

	// Open FIFO with O_RDWR so it never blocks (even if job exits during open)
	fifo, err := os.OpenFile(job.StdinPipePath(name), os.O_RDWR, 0)
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
	jobExited := make(chan int, 1)

	// Follow logs in a goroutine, signal when job exits
	go attachFollowUntilExit(os.Stdout, name, done, jobExited)

	// Read stdin and forward to FIFO
	ctrlCCount := 0
	var lastCtrlC time.Time
	buf := make([]byte, 256)
	for {
		select {
		case code := <-jobExited:
			close(done)
			writeStr(fmt.Sprintf("\r\nJob exited with code %d.\r\n", code))
			return nil
		default:
		}

		readCh := make(chan readResult, 1)
		go func() {
			n, err := os.Stdin.Read(buf)
			readCh <- readResult{n, err}
		}()

		select {
		case code := <-jobExited:
			close(done)
			writeStr(fmt.Sprintf("\r\nJob exited with code %d.\r\n", code))
			return nil
		case r := <-readCh:
			if r.err != nil {
				close(done)
				writeStr("\r\nDetached from job " + name + ".\r\n")
				return nil
			}
			n := r.n

			written := 0
			detach := false
			for i := range n {
				switch buf[i] {
				case 0x11: // Ctrl+Q — detach
					if i > written {
						fifo.Write(buf[written:i])
					}
					close(done)
					writeStr("\r\nDetached from job " + name + ".\r\n")
					return nil

				case 0x03: // Ctrl+C — interrupt / kill
					if i > written {
						fifo.Write(buf[written:i])
					}
					written = i + 1

					if time.Since(lastCtrlC) > 2*time.Second {
						ctrlCCount = 0
					}
					ctrlCCount++
					lastCtrlC = time.Now()

					if ctrlCCount >= 3 {
						signalJob(meta.PID, true)
						close(done)
						writeStr("\r\nKilled job " + name + ".\r\n")
						detach = true
					} else {
						signalJob(meta.PID, false)
						remaining := 3 - ctrlCCount
						writeStr(fmt.Sprintf("\r\nInterrupted. %d more Ctrl+C to kill.\r\n", remaining))
					}
				}
				if detach {
					break
				}
			}
			if detach {
				return nil
			}
			if written < n {
				fifo.Write(buf[written:n])
			}
		}
	}
}

func signalJob(pid int, kill bool) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if kill {
		proc.Signal(os.Kill)
	} else {
		proc.Signal(os.Interrupt)
	}
}

// attachFollowUntilExit streams log lines and signals jobExited when the job finishes.
func attachFollowUntilExit(w io.Writer, name string, stop <-chan struct{}, jobExited chan<- int) {
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
				fmt.Fprint(w, e.line+"\r\n")
			}
		}

		// Check if job has exited
		meta, err := job.ReadMeta(job.MetaPath(name))
		if err == nil && meta.Status != job.Running {
			// Drain any remaining log entries
			time.Sleep(50 * time.Millisecond)
			outs, _ := readNewLogEntries(job.StdoutLogPath(name), stdoutOffset)
			errs, _ := readNewLogEntries(job.StderrLogPath(name), stderrOffset)
			remaining := append(outs, errs...)
			if len(remaining) > 0 {
				sort.SliceStable(remaining, func(i, j int) bool {
					return remaining[i].ts < remaining[j].ts
				})
				for _, e := range remaining {
					fmt.Fprint(w, e.line+"\r\n")
				}
			}
			code := 0
			if meta.ExitCode != nil {
				code = *meta.ExitCode
			}
			jobExited <- code
			return
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
