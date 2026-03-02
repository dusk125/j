package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

func StateDir() string {
	if d := os.Getenv("J_STATE_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "j")
}

func JobsDir() string {
	return filepath.Join(StateDir(), "jobs")
}

func JobDir(name string) string {
	return filepath.Join(JobsDir(), name)
}

func MetaPath(name string) string {
	return filepath.Join(JobDir(name), "meta.json")
}

func StdoutLogPath(name string) string {
	return filepath.Join(JobDir(name), "stdout.log")
}

func StderrLogPath(name string) string {
	return filepath.Join(JobDir(name), "stderr.log")
}

func StdinPipePath(name string) string {
	return filepath.Join(JobDir(name), "stdin.pipe")
}

func SupervisorPIDPath(name string) string {
	return filepath.Join(JobDir(name), "supervisor.pid")
}

func EnsureJobsDir() error {
	return os.MkdirAll(JobsDir(), 0755)
}

func CreateJobDir(name string) error {
	return os.MkdirAll(JobDir(name), 0755)
}

func JobExists(name string) bool {
	_, err := os.Stat(JobDir(name))
	return err == nil
}

func ListJobs() ([]*Meta, error) {
	entries, err := os.ReadDir(JobsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var jobs []*Meta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := ReadMeta(MetaPath(e.Name()))
		if err != nil {
			continue
		}
		RefreshStatus(m)
		jobs = append(jobs, m)
	}
	return jobs, nil
}

// RefreshStatus checks if a running job's process is still alive and updates status.
func RefreshStatus(m *Meta) {
	if m.Status != "running" {
		return
	}
	if m.PID <= 0 {
		return
	}
	proc, err := os.FindProcess(m.PID)
	if err != nil {
		m.Status = "failed"
		return
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		// Process is dead but meta wasn't updated (supervisor crashed?)
		m.Status = "failed"
		WriteMeta(MetaPath(m.Name), m)
	}
}

func RemoveJob(name string) error {
	return os.RemoveAll(JobDir(name))
}

func WriteSupervisorPID(name string, pid int) error {
	return os.WriteFile(SupervisorPIDPath(name), []byte(fmt.Sprintf("%d", pid)), 0644)
}

func ReadSupervisorPID(name string) (int, error) {
	data, err := os.ReadFile(SupervisorPIDPath(name))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}
