//go:build !unix

package exiftool

import "os"

// watch: no watchdog here — an ExifTool left by a process that died without Close
// keeps running
func watch(*os.Process) (stop func()) { return func() {} }
