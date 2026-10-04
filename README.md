# A thin wrapper around ExifTool

[![PkgGoDev](https://pkg.go.dev/badge/image)](https://pkg.go.dev/github.com/eggs-gd/go-exiftool)

Fork of [ncruces/go-exiftool](https://github.com/ncruces/go-exiftool) adding channeled output with custom splitters (`NewServerCh`, `CommandCh`).

`Server` (ExifTool with `-stay_open`) in this fork:
- a per-command timeout (`SetTimeout`): a command that hangs kills and restarts ExifTool;
- killed processes are reaped (no zombies);
- **no orphans** (Unix): ExifTool never exits on its own at the end of its argfile — it polls for
  more, 100 times a second, forever — so a program that dies without `Close` (SIGKILL, a crash)
  would leave it running. Every ExifTool gets a watchdog: a shell blocked on a pipe whose only
  writer is the Go process; when the process dies the pipe closes and the watchdog kills ExifTool at
  once. Both are the Go process's children and reaped by it (no zombies, also as PID 1); the
  watchdog is stopped as soon as ExifTool is reaped, so it never signals a reused pid. Not on
  Windows.

This uses the excellent ExifTool by Phil Harvey:
- https://exiftool.org/ 
- https://github.com/exiftool/exiftool

[dist_unix.sh](scripts/dist_unix.sh) and [dist_windows.sh](scripts/dist_windows.sh) build minimal, self-contained distros of ExifTool
for both Unix (Linux/macOS/etc) and Windows.
Use [Git BASH](https://gitforwindows.org/) to run the Windows script.

The Windows version uses Strawberry Perl:
- https://strawberryperl.com/
- https://github.com/StrawberryPerl
