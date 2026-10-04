# A thin wrapper around ExifTool

[![PkgGoDev](https://pkg.go.dev/badge/image)](https://pkg.go.dev/github.com/eggs-gd/go-exiftool)

Fork of [ncruces/go-exiftool](https://github.com/ncruces/go-exiftool) adding channeled output with custom splitters (`NewServerCh`, `CommandCh`).

`Server` (ExifTool with `-stay_open`) in this fork:
- a per-command timeout (`SetTimeout`): a command that hangs kills and restarts ExifTool;
- killed processes are reaped (no zombies);
- **no orphans** (Unix): ExifTool never exits on its own at the end of its argfile — it polls for
  more, 100 times a second, forever — so a program that dies without `Close` (SIGKILL, a crash)
  would leave it running. ExifTool starts under `sh` with a watchdog that kills it once its parent
  is gone (within a second); the watchdog ends with ExifTool. Not on Windows.

This uses the excellent ExifTool by Phil Harvey:
- https://exiftool.org/ 
- https://github.com/exiftool/exiftool

[dist_unix.sh](scripts/dist_unix.sh) and [dist_windows.sh](scripts/dist_windows.sh) build minimal, self-contained distros of ExifTool
for both Unix (Linux/macOS/etc) and Windows.
Use [Git BASH](https://gitforwindows.org/) to run the Windows script.

The Windows version uses Strawberry Perl:
- https://strawberryperl.com/
- https://github.com/StrawberryPerl
