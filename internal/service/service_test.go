package service

import (
	"strings"
	"testing"

	"github.com/farishan-labs/processor/internal/registry"
)

func TestGenerateLaunchd(t *testing.T) {
	p := registry.Process{Name: "foo", Command: "/bin/true", Args: []string{"-x"}, AutoRestart: true, WorkDir: "/tmp"}
	out := GenerateLaunchd(p)
	if !strings.Contains(out, "<key>Label</key><string>com.fhl.processor.foo</string>") {
		t.Fatalf("missing label: %s", out)
	}
	if !strings.Contains(out, "<string>/bin/true</string>") {
		t.Fatalf("missing command")
	}
	if !strings.Contains(out, "<key>KeepAlive</key><true/>") {
		t.Fatalf("missing keepalive")
	}
}

func TestServicePath(t *testing.T) {
	p := "foo"
	path := ServicePath(p)
	if path == "" {
		t.Fatal("empty path")
	}
}