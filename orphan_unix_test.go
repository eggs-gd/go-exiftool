//go:build unix

package exiftool

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A process that dies without Close (SIGKILL) leaves no ExifTool behind: the
// watchdog kills it. The helper below is this test binary started again: it
// starts a Server, prints ExifTool's pid and kills itself.
func TestNoOrphanAfterParentDies(t *testing.T) {
	if os.Getenv("EXIFTOOL_ORPHAN_HELPER") == "1" {
		e, err := NewServer()
		if err != nil {
			fmt.Println("error", err)
			os.Exit(1)
		}
		fmt.Println(e.cmd.Process.Pid)
		syscall.Kill(os.Getpid(), syscall.SIGKILL)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestNoOrphanAfterParentDies$")
	cmd.Env = append(os.Environ(), "EXIFTOOL_ORPHAN_HELPER=1")
	out, _ := cmd.Output() // killed: an error is expected
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("helper said %q", out)
	}
	gone(t, pid, "exiftool after its parent died")
}

// Close reaps ExifTool and its watchdog: neither runs, neither is a zombie (a
// zombie still answers kill -0)
func TestCloseReapsBoth(t *testing.T) {
	e := newServer(t)
	pid, dog := e.cmd.Process.Pid, watchdogOf(t, e.cmd.Process.Pid)
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	gone(t, pid, "exiftool after Close")
	gone(t, dog, "the watchdog after Close")
}

// A restart (after a timeout or a failed command) replaces both
func TestRestartReapsBoth(t *testing.T) {
	e := newServer(t)
	defer e.Close()
	pid, dog := e.cmd.Process.Pid, watchdogOf(t, e.cmd.Process.Pid)
	if err := e.restart(); err != nil {
		t.Fatal(err)
	}
	gone(t, pid, "the old exiftool after a restart")
	gone(t, dog, "the old watchdog after a restart")
	watchdogOf(t, e.cmd.Process.Pid)
	if _, err := e.Command("-ver"); err != nil {
		t.Fatal(err)
	}
}

// ExifTool that dies by itself (a crash) takes its watchdog along at once, so the
// watchdog never signals a pid that may have been reused
func TestCrashReapsWatchdog(t *testing.T) {
	e := newServer(t)
	defer e.Close()
	dog := watchdogOf(t, e.cmd.Process.Pid)
	syscall.Kill(e.cmd.Process.Pid, syscall.SIGKILL)
	gone(t, dog, "the watchdog after exiftool crashed")
}

// An ExifTool that cannot start fails NewServer at once
func TestCannotStart(t *testing.T) {
	saved := Exec
	defer func() { Exec = saved }()
	notExecutable := filepath.Join(t.TempDir(), "exiftool")
	if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(t.TempDir(), "missing"), notExecutable} {
		Exec = path
		if e, err := NewServer(); err == nil {
			e.Close()
			t.Errorf("NewServer started %s", path)
		}
	}
}

func newServer(t *testing.T) *Server {
	t.Helper()
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Command("-ver"); err != nil {
		t.Fatal(err)
	}
	return e
}

// watchdogOf: the pid of the watchdog of ExifTool pid (a child of this process)
func watchdogOf(t *testing.T, pid int) int {
	t.Helper()
	out, _ := exec.Command("pgrep", "-P", strconv.Itoa(os.Getpid()), "-f", fmt.Sprintf(`sh %d$`, pid)).Output()
	dog, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("no watchdog for exiftool %d: %q", pid, out)
	}
	return dog
}

// gone: pid ends (and is reaped) within 3 s
func gone(t *testing.T, pid int, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("%s: %d still there", what, pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
