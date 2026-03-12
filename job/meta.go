package job

import (
	"encoding/json"
	"os"
	"time"
)

type Status string

const (
	Started Status = "started"
	Running Status = "running"
	Exited  Status = "exited"
	Failed  Status = "failed"
	Killed  Status = "killed"
)

type Meta struct {
	Name          string    `json:"name"`
	Command       []string  `json:"command"`
	Dir           string    `json:"dir"`
	PID           int       `json:"pid"`
	SupervisorPID int       `json:"supervisor_pid"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at,omitempty"`
	ExitCode      *int      `json:"exit_code,omitempty"`
	Status        Status    `json:"status"`
	AutoRemove    bool      `json:"auto_remove,omitempty"`
	ServiceUnit   string    `json:"service_unit,omitempty"`
}

func (m *Meta) IsService() bool {
	return m.ServiceUnit != ""
}

func ReadMeta(path string) (*Meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func WriteMeta(path string, m *Meta) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
