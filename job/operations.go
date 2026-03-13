package job

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StartOptions configures how a job is started.
type StartOptions struct {
	Name       string // Job name (auto-generated if empty)
	Dir        string // Working directory (default: current directory)
	Env        []string
	AutoRemove bool
	// SupervisorPath is the path to the binary that will run the supervisor.
	// Defaults to os.Executable() if empty. The binary must call
	// RunSupervisor when invoked with "_supervisor NAME" arguments.
	SupervisorPath string
}

// Start launches a new supervised background job.
func Start(command []string, opts StartOptions) (string, *Meta, error) {
	if err := EnsureJobsDir(); err != nil {
		return "", nil, fmt.Errorf("creating state directory: %w", err)
	}

	name := opts.Name
	if name == "" {
		name = GenerateName()
	}

	if JobExists(name) {
		return "", nil, fmt.Errorf("job %q already exists", name)
	}

	dir := opts.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", nil, fmt.Errorf("getting working directory: %w", err)
		}
	}

	if err := CreateJobDir(name); err != nil {
		return "", nil, fmt.Errorf("creating job directory: %w", err)
	}

	meta := &Meta{
		Name:       name,
		Command:    command,
		Dir:        dir,
		Env:        opts.Env,
		Status:     Running,
		AutoRemove: opts.AutoRemove,
	}
	if err := WriteMeta(MetaPath(name), meta); err != nil {
		return "", nil, fmt.Errorf("writing metadata: %w", err)
	}

	supervisorPath := opts.SupervisorPath
	if supervisorPath == "" {
		var err error
		supervisorPath, err = os.Executable()
		if err != nil {
			return "", nil, fmt.Errorf("finding executable: %w", err)
		}
	}

	supervisor := exec.Command(supervisorPath, "_supervisor", name)
	supervisor.Dir = dir
	supervisor.Stdin = nil
	supervisor.Stdout = nil
	supervisor.Stderr = nil
	supervisor.SysProcAttr = sysProcAttr()

	if err := supervisor.Start(); err != nil {
		RemoveJob(name)
		return "", nil, fmt.Errorf("starting supervisor: %w", err)
	}

	WriteSupervisorPID(name, supervisor.Process.Pid)

	return name, meta, nil
}

// Stop sends SIGINT to a job, waiting up to 5 seconds for graceful exit
// before sending SIGKILL.
func Stop(name string) error {
	meta, err := ReadMeta(MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	RefreshStatus(meta)
	if meta.Status != Running {
		return fmt.Errorf("job %q is not running (status: %s)", name, meta.Status)
	}

	if meta.IsService() {
		out, err := exec.Command("systemctl", "--user", "stop", meta.ServiceUnit).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl stop %s: %s", meta.ServiceUnit, strings.TrimSpace(string(out)))
		}
		return nil
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("sending SIGINT: %w", err)
	}

	timeout := 5 * time.Second
	if WaitForProcessExit(name, timeout) {
		return nil
	}

	if err := proc.Signal(os.Kill); err != nil {
		return fmt.Errorf("sending SIGKILL: %w", err)
	}

	WaitForProcessExit(name, 0)
	return nil
}

// Kill sends SIGKILL to a job.
func Kill(name string) error {
	meta, err := ReadMeta(MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	RefreshStatus(meta)
	if meta.Status != Running {
		return fmt.Errorf("job %q is not running (status: %s)", name, meta.Status)
	}

	if meta.IsService() {
		out, err := exec.Command("systemctl", "--user", "kill", "--signal=SIGKILL", meta.ServiceUnit).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl kill %s: %s", meta.ServiceUnit, strings.TrimSpace(string(out)))
		}
		return nil
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}

	if err := proc.Signal(os.Kill); err != nil {
		return fmt.Errorf("sending SIGKILL: %w", err)
	}

	WaitForProcessExit(name, 0)
	return nil
}

// Remove removes a job. If force is true, it will stop a running job first.
func Remove(name string, force bool) error {
	meta, err := ReadMeta(MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	if meta.IsService() {
		return RemoveJob(name)
	}

	if meta.Status == Running && !force {
		return fmt.Errorf("job %q is still running (use force to remove)", name)
	}

	if meta.Status == Running && force {
		RefreshStatus(meta)
		if meta.Status == Running {
			if proc, err := os.FindProcess(meta.PID); err == nil {
				proc.Signal(os.Interrupt)
				timeout := 5 * time.Second
				if !WaitForProcessExit(name, timeout) {
					proc.Signal(os.Kill)
					WaitForProcessExit(name, 0)
				}
			}
		}
	}

	return RemoveJob(name)
}

// Restart stops a running job (if needed) and restarts it with the same configuration.
// Returns the new job name and metadata.
func Restart(name string, opts *StartOptions) (string, *Meta, error) {
	meta, err := ReadMeta(MetaPath(name))
	if err != nil {
		return "", nil, fmt.Errorf("job %q not found", name)
	}

	if meta.IsService() {
		out, err := exec.Command("systemctl", "--user", "restart", meta.ServiceUnit).CombinedOutput()
		if err != nil {
			return "", nil, fmt.Errorf("systemctl restart %s: %s", meta.ServiceUnit, strings.TrimSpace(string(out)))
		}
		RefreshStatus(meta)
		return name, meta, nil
	}

	if meta.Status == Running {
		RefreshStatus(meta)
	}
	if meta.Status == Running {
		proc, err := os.FindProcess(meta.PID)
		if err != nil {
			return "", nil, fmt.Errorf("finding process: %w", err)
		}
		proc.Signal(os.Interrupt)

		for range 50 {
			time.Sleep(100 * time.Millisecond)
			meta, _ = ReadMeta(MetaPath(name))
			if meta.Status != Running {
				break
			}
			RefreshStatus(meta)
			if meta.Status != Running {
				break
			}
		}

		if meta.Status == Running {
			proc.Signal(os.Kill)
			time.Sleep(200 * time.Millisecond)
		}
	}

	command := meta.Command
	dir := meta.Dir
	env := meta.Env
	autoRemove := meta.AutoRemove

	if err := RemoveJob(name); err != nil {
		return "", nil, fmt.Errorf("removing old job: %w", err)
	}

	startOpts := StartOptions{
		Name:       name,
		Dir:        dir,
		Env:        env,
		AutoRemove: autoRemove,
	}
	if opts != nil {
		startOpts.SupervisorPath = opts.SupervisorPath
	}

	return Start(command, startOpts)
}

// Inspect returns the current metadata for a job with refreshed status.
func Inspect(name string) (*Meta, error) {
	meta, err := ReadMeta(MetaPath(name))
	if err != nil {
		return nil, fmt.Errorf("job %q not found", name)
	}
	RefreshStatus(meta)
	return meta, nil
}

// Rename renames a job from oldName to newName.
func Rename(oldName, newName string) error {
	if oldName == newName {
		return fmt.Errorf("old and new names are the same")
	}

	meta, err := ReadMeta(MetaPath(oldName))
	if err != nil {
		return fmt.Errorf("job %q not found", oldName)
	}

	RefreshStatus(meta)

	if JobExists(newName) {
		return fmt.Errorf("job %q already exists", newName)
	}

	oldDir := JobDir(oldName)
	newDir := JobDir(newName)

	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("renaming job directory: %w", err)
	}

	meta.Name = newName
	if err := WriteMeta(MetaPath(newName), meta); err != nil {
		os.Rename(newDir, oldDir)
		return fmt.Errorf("updating metadata: %w", err)
	}

	if meta.Status == Running {
		rel, err := filepath.Rel(filepath.Dir(oldDir), newDir)
		if err != nil {
			rel = newDir
		}
		os.Symlink(rel, oldDir)
	}

	return nil
}

// Clean removes all non-running, non-service jobs.
// Returns the names of removed jobs.
func Clean() ([]string, error) {
	jobs, err := ListJobs()
	if err != nil {
		return nil, err
	}

	var removed []string
	for _, m := range jobs {
		if m.Status == Running || m.IsService() {
			continue
		}
		if err := RemoveJob(m.Name); err != nil {
			continue
		}
		removed = append(removed, m.Name)
	}
	return removed, nil
}

// Wait blocks until a job exits and returns its exit code.
func Wait(name string) (int, error) {
	meta, err := ReadMeta(MetaPath(name))
	if err != nil {
		return 0, fmt.Errorf("job %q not found", name)
	}

	if meta.Status != Running {
		return exitCode(meta), nil
	}

	for {
		time.Sleep(100 * time.Millisecond)
		meta, err = ReadMeta(MetaPath(name))
		if err != nil {
			return 0, fmt.Errorf("reading job state: %w", err)
		}
		RefreshStatus(meta)
		if meta.Status != Running {
			return exitCode(meta), nil
		}
	}
}

func exitCode(meta *Meta) int {
	if meta.ExitCode != nil {
		return *meta.ExitCode
	}
	return 0
}

// Manage registers an existing systemctl user service as a j job.
func Manage(unit string, name string) (*Meta, error) {
	if !strings.HasSuffix(unit, ".service") {
		unit = unit + ".service"
	}

	if err := exec.Command("systemctl", "--user", "cat", unit).Run(); err != nil {
		return nil, fmt.Errorf("service %q not found (systemctl --user cat %s failed)", unit, unit)
	}

	if name == "" {
		name = strings.TrimSuffix(unit, ".service")
	}

	if JobExists(name) {
		return nil, fmt.Errorf("job %q already exists", name)
	}

	if err := EnsureJobsDir(); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}
	if err := CreateJobDir(name); err != nil {
		return nil, fmt.Errorf("creating job directory: %w", err)
	}

	meta := &Meta{
		Name:        name,
		Command:     []string{unit},
		ServiceUnit: unit,
		Status:      Running,
	}
	RefreshStatus(meta)

	if err := WriteMeta(MetaPath(name), meta); err != nil {
		return nil, fmt.Errorf("writing metadata: %w", err)
	}

	return meta, nil
}

// Unmanage removes a managed systemctl service from j without affecting the service.
func Unmanage(name string) (string, error) {
	meta, err := ReadMeta(MetaPath(name))
	if err != nil {
		return "", fmt.Errorf("job %q not found", name)
	}
	if !meta.IsService() {
		return "", fmt.Errorf("job %q is not a managed service", name)
	}

	unit := meta.ServiceUnit
	if err := RemoveJob(name); err != nil {
		return "", fmt.Errorf("removing job: %w", err)
	}

	return unit, nil
}

// WaitForProcessExit polls until the job is no longer running.
// If timeout is 0, it waits indefinitely.
// Returns true if the process exited within the timeout.
func WaitForProcessExit(name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		meta, err := ReadMeta(MetaPath(name))
		if err != nil {
			return true
		}
		RefreshStatus(meta)
		if meta.Status != Running {
			return true
		}
		if timeout > 0 && time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// SignalJob sends an interrupt or kill signal to a process by PID.
func SignalJob(pid int, kill bool) {
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
