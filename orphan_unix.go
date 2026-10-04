//go:build unix

package exiftool

import "os/exec"

// watchdog: ExifTool with -stay_open never exits on its own — at the end of its
// argfile (stdin) it polls for more, 100 times a second, forever. A Go program that
// dies without Close (SIGKILL, a crash) would leave it running. So ExifTool starts
// under sh: a background watcher kills it once its parent (this process) is gone,
// and ends by itself once ExifTool is. `exec` makes ExifTool take sh's pid, so
// Kill and Wait reach ExifTool itself; the watcher holds none of its pipes.
const watchdog = `parent=$PPID
(while kill -0 "$parent" && kill -0 $$; do sleep 1; done; kill -9 $$) </dev/null >/dev/null 2>&1 &
exec "$0" "$@"`

// command: ExifTool as a child that does not outlive this process
func command(name string, args ...string) *exec.Cmd {
	return exec.Command("/bin/sh", append([]string{"-c", watchdog, name}, args...)...)
}
