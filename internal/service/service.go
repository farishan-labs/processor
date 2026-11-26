package service

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/farishan-labs/processor/internal/registry"
)

func ServiceName(name string) string { return "com.fhl.processor." + name }

func ServicePath(name string) string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "LaunchAgents", ServiceName(name)+".plist")
	}
	return filepath.Join(home, ".config", "systemd", "user", name+".service")
}

func logPath(p registry.Process) string {
	if p.LogPath != "" {
		return p.LogPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "fhl", "logs", p.Name+".log")
}

func GenerateLaunchd(p registry.Process) string {
	lp := logPath(p)
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	sb.WriteString(`<plist version="1.0"><dict>` + "\n")
	sb.WriteString(fmt.Sprintf("  <key>Label</key><string>%s</string>\n", ServiceName(p.Name)))
	sb.WriteString("  <key>ProgramArguments</key><array>\n")
	sb.WriteString(fmt.Sprintf("    <string>%s</string>\n", p.Command))
	for _, a := range p.Args {
		sb.WriteString(fmt.Sprintf("    <string>%s</string>\n", a))
	}
	sb.WriteString("  </array>\n")
	if p.WorkDir != "" {
		sb.WriteString(fmt.Sprintf("  <key>WorkingDirectory</key><string>%s</string>\n", p.WorkDir))
	}
	if len(p.Env) > 0 {
		sb.WriteString("  <key>EnvironmentVariables</key><dict>\n")
		for k, v := range p.Env {
			sb.WriteString(fmt.Sprintf("    <key>%s</key><string>%s</string>\n", k, v))
		}
		sb.WriteString("  </dict>\n")
	}
	sb.WriteString("  <key>RunAtLoad</key><true/>\n")
	if p.AutoRestart {
		sb.WriteString("  <key>KeepAlive</key><true/>\n")
	}
	sb.WriteString(fmt.Sprintf("  <key>StandardOutPath</key><string>%s</string>\n", lp))
	sb.WriteString(fmt.Sprintf("  <key>StandardErrorPath</key><string>%s</string>\n", lp))
	sb.WriteString("</dict></plist>\n")
	return sb.String()
}

func GenerateSystemd(p registry.Process) string {
	lp := logPath(p)
	var sb strings.Builder
	sb.WriteString("[Unit]\n")
	sb.WriteString(fmt.Sprintf("Description=FHL processor: %s\n", p.Name))
	sb.WriteString("After=network.target\n\n")
	sb.WriteString("[Service]\n")
	sb.WriteString("Type=simple\n")
	sb.WriteString(fmt.Sprintf("ExecStart=%s %s\n", p.Command, strings.Join(p.Args, " ")))
	if p.WorkDir != "" {
		sb.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", p.WorkDir))
	}
	for k, v := range p.Env {
		sb.WriteString(fmt.Sprintf("Environment=%s=%s\n", k, v))
	}
	if p.AutoRestart {
		sb.WriteString("Restart=always\n")
	}
	sb.WriteString(fmt.Sprintf("StandardOutput=append:%s\n", lp))
	sb.WriteString(fmt.Sprintf("StandardError=append:%s\n", lp))
	sb.WriteString("\n[Install]\n")
	sb.WriteString("WantedBy=default.target\n")
	return sb.String()
}

func Generate(p registry.Process) string {
	if runtime.GOOS == "darwin" {
		return GenerateLaunchd(p)
	}
	return GenerateSystemd(p)
}

func Install(p registry.Process) error {
	path := ServicePath(p.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath(p)), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(Generate(p)), 0644); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		// Prefer modern bootstrap; fall back to legacy load.
		if err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), path).Run(); err == nil {
			return nil
		}
		return exec.Command("launchctl", "load", path).Run()
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return exec.Command("systemctl", "--user", "enable", p.Name+".service").Run()
}

func Uninstall(name string) error {
	path := ServicePath(name)
	if runtime.GOOS == "darwin" {
		if err := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), path).Run(); err != nil {
			_ = exec.Command("launchctl", "unload", path).Run()
		}
	} else {
		_ = exec.Command("systemctl", "--user", "disable", name+".service").Run()
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	}
	return os.Remove(path)
}

func Start(name string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("launchctl", "start", ServiceName(name)).Run()
	}
	return exec.Command("systemctl", "--user", "start", name+".service").Run()
}

func Stop(name string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("launchctl", "stop", ServiceName(name)).Run()
	}
	return exec.Command("systemctl", "--user", "stop", name+".service").Run()
}

func Restart(name string) error {
	_ = Stop(name)
	return Start(name)
}

func DiscoverOwned() ([]registry.Process, error) {
	if runtime.GOOS != "darwin" {
		return nil, nil
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Library", "LaunchAgents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []registry.Process
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		label := strings.TrimSuffix(e.Name(), ".plist")
		if !(strings.HasPrefix(label, "com.faris.") || strings.HasPrefix(label, "com.fhl.")) {
			continue
		}
		p, err := parsePlist(filepath.Join(dir, e.Name()))
		if err != nil || p.Command == "" {
			continue
		}
		// friendly name: strip prefix
		name := label
		name = strings.TrimPrefix(name, "com.faris.")
		name = strings.TrimPrefix(name, "com.fhl.processor.")
		name = strings.TrimPrefix(name, "com.fhl.")
		p.Name = name
		out = append(out, p)
	}
	return out, nil
}

func Adopt(label string) (registry.Process, error) {
	if runtime.GOOS != "darwin" {
		return registry.Process{}, fmt.Errorf("adopt only supported on macOS currently")
	}
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "LaunchAgents", label+".plist")
	p, err := parsePlist(path)
	if err != nil {
		return registry.Process{}, err
	}
	name := label
	name = strings.TrimPrefix(name, "com.faris.")
	name = strings.TrimPrefix(name, "com.fhl.processor.")
	name = strings.TrimPrefix(name, "com.fhl.")
	p.Name = name
	return p, nil
}

func parsePlist(path string) (registry.Process, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return registry.Process{}, err
	}
	s := string(data)
	labelRe := regexp.MustCompile(`<key>Label</key>\s*<string>([^<]+)</string>`)
	lm := labelRe.FindStringSubmatch(s)
	var label string
	if len(lm) > 1 {
		label = lm[1]
	}
	// ProgramArguments array
	args := []string{}
	if idx := strings.Index(s, "<key>ProgramArguments</key>"); idx >= 0 {
		rest := s[idx:]
		start := strings.Index(rest, "<array>")
		end := strings.Index(rest, "</array>")
		if start >= 0 && end > start {
			block := rest[start+len("<array>") : end]
			strRe := regexp.MustCompile(`<string>([^<]*)</string>`)
			for _, m := range strRe.FindAllStringSubmatch(block, -1) {
				args = append(args, m[1])
			}
		}
	}
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}
	// WorkingDirectory
	wd := ""
	wdRe := regexp.MustCompile(`<key>WorkingDirectory</key>\s*<string>([^<]+)</string>`)
	if m := wdRe.FindStringSubmatch(s); len(m) > 1 {
		wd = m[1]
	}
	// StandardOutPath
	lp := ""
	lpRe := regexp.MustCompile(`<key>StandardOutPath</key>\s*<string>([^<]+)</string>`)
	if m := lpRe.FindStringSubmatch(s); len(m) > 1 {
		lp = m[1]
	}
	// best-effort Type
	ptype := "daemon"
	lowerLabel := strings.ToLower(label)
	if strings.Contains(lowerLabel, "menubar") || strings.Contains(lowerLabel, "tray") {
		ptype = "menubar"
	} else if strings.Contains(lowerLabel, "job") {
		ptype = "job"
	}
	// best-effort HealthURL: scan args for http(s) URL
	healthURL := ""
	for _, a := range args {
		if strings.HasPrefix(a, "http://") || strings.HasPrefix(a, "https://") {
			healthURL = a
			break
		}
	}
	_ = label
	return registry.Process{Command: cmd, Args: args, WorkDir: wd, LogPath: lp, Type: ptype, HealthURL: healthURL}, nil
}

func Status(name string) (string, error) {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("launchctl", "list", ServiceName(name)).CombinedOutput()
		return string(out), err
	}
	out, err := exec.Command("systemctl", "--user", "status", name+".service", "--no-pager").CombinedOutput()
	return string(out), err
}