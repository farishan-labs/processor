package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Process struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	WorkDir     string            `json:"workdir,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Type        string            `json:"type,omitempty"`        // daemon|menubar|job
	HealthURL   string            `json:"health_url,omitempty"`
	LogPath     string            `json:"log_path,omitempty"`
	AutoRestart bool              `json:"auto_restart,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

type Registry struct {
	Processes []Process `json:"processes"`
	Path      string    `json:"-"`
}

func DefaultPath() string {
	if v := os.Getenv("PROCESSOR_REGISTRY"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "fhl", "processor.json")
}

func Load(path string) (*Registry, error) {
	r := &Registry{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, err
	}
	r.Path = path
	return r, nil
}

func (r *Registry) Save() error {
	if r.Path == "" {
		r.Path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(r.Path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.Path, data, 0644)
}

func (r *Registry) Add(p Process) error {
	for _, existing := range r.Processes {
		if existing.Name == p.Name {
			return fmt.Errorf("process %q already exists", p.Name)
		}
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	r.Processes = append(r.Processes, p)
	return r.Save()
}

func (r *Registry) Get(name string) (Process, bool) {
	for _, p := range r.Processes {
		if p.Name == name {
			return p, true
		}
	}
	return Process{}, false
}

func (r *Registry) Remove(name string) error {
	for i, p := range r.Processes {
		if p.Name == name {
			r.Processes = append(r.Processes[:i], r.Processes[i+1:]...)
			return r.Save()
		}
	}
	return fmt.Errorf("process %q not found", name)
}