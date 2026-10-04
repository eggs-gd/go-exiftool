//go:build unix

package exiftool

import (
	"os"
	"os/exec"
	"strconv"
)

// watch: ExifTool with -stay_open never exits on its own — at the end of its
// argfile (stdin) it polls for more, 100 times a second, forever. A Go program that
// dies without Close (SIGKILL, a crash) would leave it running. So every ExifTool
// gets a watchdog: a shell blocked reading a pipe whose only writer is this
// process. When the process dies, the kernel closes the writer, the read ends and
// the watchdog kills ExifTool — at once, no polling.
//
// Both are this process's children, so it reaps both (no zombies, also as PID 1
// in a container). The returned stop kills the watchdog before it could act and
// reaps it; it runs as soon as ExifTool is reaped, so the watchdog never signals a
// pid that may have been reused. Without a pipe or a shell there is no watchdog:
// ExifTool runs as before.
func watch(exiftool *os.Process) (stop func()) {
	r, w, err := os.Pipe() // close-on-exec: no other child inherits the writer
	if err != nil {
		return func() {}
	}
	dog := exec.Command("/bin/sh", "-c", `read _; kill -9 "$1"`, "sh", strconv.Itoa(exiftool.Pid))
	dog.Stdin = r
	if err := dog.Start(); err != nil {
		r.Close()
		w.Close()
		return func() {}
	}
	r.Close() // the watchdog holds it now
	return func() {
		dog.Process.Kill()
		dog.Wait()
		w.Close() // only now: closing it first would set the watchdog off
	}
}
