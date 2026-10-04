//go:build !unix

package exiftool

import "os/exec"

// command: ExifTool as a child (no watchdog here: an ExifTool left by a process
// that died without Close keeps running)
func command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
