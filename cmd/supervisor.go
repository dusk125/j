package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
)

var supervisorCmd = &cobra.Command{
	Use:    "_supervisor NAME",
	Short:  "Internal: supervise a job process",
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE:   runSupervisor,
}

func runSupervisor(cmd *cobra.Command, args []string) error {
	name := args[0]

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("reading meta: %w", err)
	}

	stdoutLog, err := job.NewLogWriter(job.StdoutLogPath(name))
	if err != nil {
		return fmt.Errorf("creating stdout log: %w", err)
	}
	defer stdoutLog.Close()

	stderrLog, err := job.NewLogWriter(job.StderrLogPath(name))
	if err != nil {
		return fmt.Errorf("creating stderr log: %w", err)
	}
	defer stderrLog.Close()

	// Create a FIFO for stdin so attach clients can send input
	fifoPath := job.StdinPipePath(name)
	os.Remove(fifoPath) // clean up any stale FIFO
	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		return fmt.Errorf("creating stdin fifo: %w", err)
	}
	// Open O_RDWR so reads block (instead of EOF) when no writer is connected
	stdinFifo, err := os.OpenFile(fifoPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("opening stdin fifo: %w", err)
	}
	defer stdinFifo.Close()

	child := exec.Command(meta.Command[0], meta.Command[1:]...)
	child.Dir = meta.Dir
	child.Stdin = stdinFifo

	stdoutPipe, err := child.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderrPipe, err := child.StderrPipe()
	if err != nil {
		return fmt.Errorf("creating stderr pipe: %w", err)
	}

	now := time.Now()
	meta.StartedAt = now

	if err := child.Start(); err != nil {
		meta.Status = "failed"
		meta.EndedAt = time.Now()
		job.WriteMeta(job.MetaPath(name), meta)
		return fmt.Errorf("starting command: %w", err)
	}

	meta.PID = child.Process.Pid
	meta.SupervisorPID = os.Getpid()
	job.WriteMeta(job.MetaPath(name), meta)

	// Copy output in goroutines
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(stdoutLog, stdoutPipe)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(stderrLog, stderrPipe)
		done <- struct{}{}
	}()

	// Wait for both pipes to close
	<-done
	<-done

	err = child.Wait()
	meta.EndedAt = time.Now()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			meta.ExitCode = &code
			meta.Status = "exited"
		} else {
			meta.Status = "failed"
		}
	} else {
		code := 0
		meta.ExitCode = &code
		meta.Status = "exited"
	}

	job.WriteMeta(job.MetaPath(name), meta)

	if meta.AutoRemove {
		job.RemoveJob(name)
	}

	return nil
}
