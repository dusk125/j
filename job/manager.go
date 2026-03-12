package job

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	if m.IsService() {
		refreshServiceStatus(m)
		return
	}
	if m.Status != Running {
		return
	}
	if m.PID <= 0 {
		return
	}
	proc, err := os.FindProcess(m.PID)
	if err != nil {
		m.Status = Failed
		return
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		// Process is dead but meta wasn't updated (supervisor crashed?)
		m.Status = Failed
		WriteMeta(MetaPath(m.Name), m)
	}
}

// refreshServiceStatus queries systemctl for the current state of a managed service.
func refreshServiceStatus(m *Meta) {
	out, err := exec.Command("systemctl", "--user", "show", m.ServiceUnit,
		"--property=ActiveState,MainPID").Output()
	if err != nil {
		m.Status = Failed
		return
	}
	props := parseSystemctlProperties(string(out))

	switch props["ActiveState"] {
	case "active", "reloading", "activating":
		m.Status = Running
	case "inactive", "deactivating":
		m.Status = Exited
	default:
		m.Status = Failed
	}

	if pid, err := strconv.Atoi(props["MainPID"]); err == nil && pid > 0 {
		m.PID = pid
	} else {
		m.PID = 0
	}
}

func parseSystemctlProperties(output string) map[string]string {
	props := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			props[k] = v
		}
	}
	return props
}

func RemoveJob(name string) error {
	if err := os.RemoveAll(JobDir(name)); err != nil {
		return err
	}
	CleanDanglingSymlinks()
	return nil
}

// CleanDanglingSymlinks removes any symlinks in the jobs directory that
// no longer point to a valid target (left over from job renames).
func CleanDanglingSymlinks() {
	entries, err := os.ReadDir(JobsDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink == 0 {
			continue
		}
		p := filepath.Join(JobsDir(), e.Name())
		if _, err := os.Stat(p); os.IsNotExist(err) {
			os.Remove(p)
		}
	}
}

func WriteSupervisorPID(name string, pid int) error {
	return os.WriteFile(SupervisorPIDPath(name), fmt.Appendf(nil, "%d", pid), 0644)
}

func ReadSupervisorPID(name string) (int, error) {
	data, err := os.ReadFile(SupervisorPIDPath(name))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}
