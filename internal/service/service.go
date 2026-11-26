package service

import (
	"fmt"
	"os"
	"os/exec"
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

func Status(name string) (string, error) {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("launchctl", "list", ServiceName(name)).CombinedOutput()
		return string(out), err
	}
	out, err := exec.Command("systemctl", "--user", "status", name+".service", "--no-pager").CombinedOutput()
	return string(out), err
}