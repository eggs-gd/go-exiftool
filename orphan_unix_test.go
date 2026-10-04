//go:build unix

package exiftool

import (
	"fmt"
	"os"
	"os/exec"
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
	deadline := time.Now().Add(5 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("exiftool %d still runs 5 s after its parent died", pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Close leaves nothing either: ExifTool is reaped, its watchdog ends by itself
func TestCloseEndsWatchdog(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Command("-ver"); err != nil {
		t.Fatal(err)
	}
	// The watchdog is ExifTool's child (sh forked it, then became ExifTool)
	out, _ := exec.Command("pgrep", "-P", strconv.Itoa(e.cmd.Process.Pid)).Output()
	watcher, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("no watchdog under exiftool: %q", out)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for syscall.Kill(watcher, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("the watchdog %d still runs after Close", watcher)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
