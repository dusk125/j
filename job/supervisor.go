package job

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// RunSupervisor runs the supervisor process for a named job.
// This should be called by the host binary when invoked with
// "_supervisor NAME" arguments.
func RunSupervisor(name string) error {
	meta, err := ReadMeta(MetaPath(name))
	if err != nil {
		return fmt.Errorf("reading meta: %w", err)
	}

	stdoutLog, err := NewLogWriter(StdoutLogPath(name))
	if err != nil {
		return fmt.Errorf("creating stdout log: %w", err)
	}
	defer stdoutLog.Close()

	stderrLog, err := NewLogWriter(StderrLogPath(name))
	if err != nil {
		return fmt.Errorf("creating stderr log: %w", err)
	}
	defer stderrLog.Close()

	fifoPath := StdinPipePath(name)
	os.Remove(fifoPath)
	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		return fmt.Errorf("creating stdin fifo: %w", err)
	}
	stdinFifo, err := os.OpenFile(fifoPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("opening stdin fifo: %w", err)
	}
	defer stdinFifo.Close()

	child := exec.Command(meta.Command[0], meta.Command[1:]...)
	child.Dir = meta.Dir
	if len(meta.Env) > 0 {
		child.Env = append(os.Environ(), meta.Env...)
	}
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
		meta.Status = Failed
		meta.EndedAt = time.Now()
		WriteMeta(MetaPath(name), meta)
		return fmt.Errorf("starting command: %w", err)
	}

	meta.PID = child.Process.Pid
	meta.SupervisorPID = os.Getpid()
	WriteMeta(MetaPath(name), meta)

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(stdoutLog, stdoutPipe)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(stderrLog, stderrPipe)
		done <- struct{}{}
	}()

	<-done
	<-done

	err = child.Wait()
	meta.EndedAt = time.Now()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			meta.ExitCode = &code
			meta.Status = Exited
		} else {
			meta.Status = Failed
		}
	} else {
		code := 0
		meta.ExitCode = &code
		meta.Status = Exited
	}

	WriteMeta(MetaPath(name), meta)

	if meta.AutoRemove {
		RemoveJob(name)
	}

	return nil
}
