package exiftool

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A fake "exiftool" that never answers, like ExifTool stuck on a broken file.
func hangingExec(t *testing.T) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "hang.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexec sleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func TestCommandTimeout(t *testing.T) {
	saved := Exec
	Exec = hangingExec(t)
	defer func() { Exec = saved }()

	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	e.SetTimeout(200 * time.Millisecond)

	first := e.cmd.Process.Pid
	start := time.Now()
	_, err = e.Command("-ver")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("timeout took %v", d)
	}
	if e.cmd.Process.Pid == first {
		t.Fatal("process was not restarted after the timeout")
	}

	// the server keeps working (and keeps timing out) after the restart
	if _, err = e.Command("-ver"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("second command: want ErrTimeout, got %v", err)
	}
}

func TestCloseReapsProcess(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if e.cmd.ProcessState == nil {
		t.Fatal("process was not waited for (zombie)")
	}
}

func TestCommandKeepsOutputOnStderrError(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	// one existing file and one missing: stderr reports the missing one,
	// stdout still has the data of the existing one
	out, err := e.Command("-ver", "-FileName", "server.go", "does-not-exist.jpg")
	if err == nil {
		t.Fatal("want an error for the missing file")
	}
	if len(out) == 0 {
		t.Fatalf("stdout dropped on stderr error: %v", err)
	}
}
