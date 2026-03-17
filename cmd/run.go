package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var runCmd = &cobra.Command{
	Use:   "run [flags] -- CMD [ARGS...]",
	Short: "Run a job in the foreground",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRun,
}

var runName string
var runDir string
var runAutoRm bool
var runEnv []string

func init() {
	runCmd.Flags().StringVarP(&runName, "name", "n", "", "Job name (auto-generated if empty)")
	runCmd.Flags().StringVarP(&runDir, "dir", "d", "", "Working directory (default: current directory)")
	runCmd.Flags().BoolVar(&runAutoRm, "rm", false, "Remove job after it exits")
	runCmd.Flags().StringArrayVarP(&runEnv, "env", "e", nil, "Set environment variables (KEY=VALUE)")
}

func runRun(cmd *cobra.Command, args []string) error {
	name, _, err := startJob(runName, runDir, false, runEnv, args)
	if err != nil {
		return err
	}

	// Wait briefly for supervisor to set PID and create FIFO
	time.Sleep(50 * time.Millisecond)

	fd := int(os.Stdin.Fd())
	isTTY := term.IsTerminal(fd)

	var result error
	if isTTY {
		result = foregroundAttach(name)
	} else {
		result = foregroundFollow(name)
	}

	if runAutoRm {
		job.RemoveJob(name)
	}

	return result
}

// foregroundAttach runs like attach but exits when the job does.
func foregroundAttach(name string) error {
	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	// Open FIFO with O_RDWR so it never blocks (even if supervisor already exited)
	fifo, err := os.OpenFile(job.StdinPipePath(name), os.O_RDWR, 0)
	if err != nil {
		// FIFO gone — job exited before we got here, fall back to drain
		return foregroundFollow(name)
	}
	defer fifo.Close()

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("setting raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	writeStr := func(s string) {
		os.Stdout.WriteString(s)
	}

	done := make(chan struct{})
	jobExited := make(chan int, 1)

	// Follow logs, exit when job exits
	go attachFollowUntilExit(os.Stdout, name, done, jobExited)

	// Read stdin and forward to FIFO
	ctrlCCount := 0
	var lastCtrlC time.Time
	buf := make([]byte, 256)
	for {
		// Check if job exited between reads
		select {
		case code := <-jobExited:
			close(done)
			if code != 0 {
				writeStr(fmt.Sprintf("\r\nJob exited with code %d.\r\n", code))
			}
			return exitCodeError{code}
		default:
		}

		// Use a goroutine for non-blocking read so we can check jobExited
		readCh := make(chan readResult, 1)
		go func() {
			n, err := os.Stdin.Read(buf)
			readCh <- readResult{n, err}
		}()

		select {
		case code := <-jobExited:
			close(done)
			if code != 0 {
				writeStr(fmt.Sprintf("\r\nJob exited with code %d.\r\n", code))
			}
			return exitCodeError{code}
		case r := <-readCh:
			if r.err != nil {
				close(done)
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

// foregroundFollow follows logs without stdin forwarding (non-TTY mode).
func foregroundFollow(name string) error {
	done := make(chan struct{})
	jobExited := make(chan int, 1)

	go attachFollowUntilExit(os.Stdout, name, done, jobExited)

	code := <-jobExited
	close(done)
	if code != 0 {
		fmt.Fprintf(os.Stderr, "Job exited with code %d.\n", code)
	}
	return exitCodeError{code}
}

type readResult struct {
	n   int
	err error
}
