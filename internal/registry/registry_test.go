package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryCRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "processor.json")
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Add(Process{Name: "foo", Command: "/bin/true"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get("foo"); !ok {
		t.Fatal("expected foo")
	}
	if err := r.Add(Process{Name: "foo", Command: "/bin/true"}); err == nil {
		t.Fatal("expected duplicate error")
	}
	if err := r.Remove("foo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get("foo"); ok {
		t.Fatal("expected removed")
	}
	// ensure file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}