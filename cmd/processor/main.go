package main

import (
	"fmt"
	"os"
	"strings"

	"os/exec"

	"github.com/farishan-labs/processor/internal/registry"
	"github.com/farishan-labs/processor/internal/service"
	"github.com/farishan-labs/processor/internal/version"
)

func usage() {
	fmt.Println(`processor - manage owned processes as services

Commands:
  version
  list
  adopt <label> [--name <name>]
  add <name> --cmd <command> [--arg <arg>]... [--workdir <dir>] [--log <path>] [--autorestart]
  start <name>
  stop <name>
  restart <name>
  remove <name>
  status <name>
  logs <name>`)
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	regPath := registry.DefaultPath()
	reg, err := registry.Load(regPath)
	if err != nil {
		fail(err.Error())
	}

	switch os.Args[1] {
	case "version":
		fmt.Println("processor", version.Version)
	case "list":
		if len(reg.Processes) == 0 {
			// auto-adopt owned launchd agents on first run
			adopted, _ := service.DiscoverOwned()
			for _, p := range adopted {
				_ = reg.Add(p)
			}
		}
		if len(reg.Processes) == 0 {
			fmt.Println("no processes registered")
			return
		}
		for _, p := range reg.Processes {
			fmt.Printf("%-20s %s\n", p.Name, p.Command)
		}
	case "add":
		if len(os.Args) < 3 {
			fail("missing name")
		}
		name := os.Args[2]
		cmd := ""
		args := []string{}
		workdir := ""
		logPath := ""
		auto := false
		for i := 3; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--cmd":
				if i+1 < len(os.Args) {
					cmd = os.Args[i+1]
					i++
				}
			case "--arg":
				if i+1 < len(os.Args) {
					args = append(args, os.Args[i+1])
					i++
				}
			case "--workdir":
				if i+1 < len(os.Args) {
					workdir = os.Args[i+1]
					i++
				}
			case "--log":
				if i+1 < len(os.Args) {
					logPath = os.Args[i+1]
					i++
				}
			case "--autorestart":
				auto = true
			}
		}
		if cmd == "" {
			fail("--cmd required")
		}
		if _, err := exec.LookPath(cmd); err != nil {
			fail("command not found on PATH: " + cmd)
		}
		if err := reg.Add(registry.Process{Name: name, Command: cmd, Args: args, WorkDir: workdir, LogPath: logPath, AutoRestart: auto}); err != nil {
			fail(err.Error())
		}
		fmt.Println("added", name)
	case "start":
		if len(os.Args) < 3 {
			fail("missing name")
		}
		name := os.Args[2]
		p, ok := reg.Get(name)
		if !ok {
			fail("not found")
		}
		if err := service.Install(p); err != nil {
			fail(err.Error())
		}
		if err := service.Start(name); err != nil {
			fail(err.Error())
		}
		fmt.Println("started", name)
	case "stop":
		if len(os.Args) < 3 {
			fail("missing name")
		}
		name := os.Args[2]
		if err := service.Stop(name); err != nil {
			fail(err.Error())
		}
		fmt.Println("stopped", name)
	case "restart":
		if len(os.Args) < 3 {
			fail("missing name")
		}
		name := os.Args[2]
		if err := service.Restart(name); err != nil {
			fail(err.Error())
		}
		fmt.Println("restarted", name)
	case "remove":
		if len(os.Args) < 3 {
			fail("missing name")
		}
		name := os.Args[2]
		_ = service.Uninstall(name)
		if err := reg.Remove(name); err != nil {
			fail(err.Error())
		}
		fmt.Println("removed", name)
	case "adopt":
		if len(os.Args) < 3 {
			fail("missing label")
		}
		label := os.Args[2]
		name := label
		if len(os.Args) >= 5 && os.Args[3] == "--name" {
			name = os.Args[4]
		}
		p, err := service.Adopt(label)
		if err != nil {
			fail(err.Error())
		}
		p.Name = name
		if err := reg.Add(p); err != nil {
			fail(err.Error())
		}
		fmt.Println("adopted", name)
	case "status":
		if len(os.Args) < 3 {
			fail("missing name")
		}
		name := os.Args[2]
		out, err := service.Status(name)
		fmt.Print(out)
		if err != nil {
			fail(err.Error())
		}
	case "logs":
		if len(os.Args) < 3 {
			fail("missing name")
		}
		name := os.Args[2]
		p, ok := reg.Get(name)
		if !ok {
			fail("not found")
		}
		lp := p.LogPath
		if lp == "" {
			home, _ := os.UserHomeDir()
			lp = home + "/.config/fhl/logs/" + name + ".log"
		}
		data, err := os.ReadFile(lp)
		if err != nil {
			fail(err.Error())
		}
		fmt.Print(string(data))
	default:
		usage()
	}
	_ = strings.TrimSpace
}