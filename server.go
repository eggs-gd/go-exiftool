package exiftool

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os/exec"
	"sync"
	"time"
)

const boundary = "1854673209"

var endPattern = []byte("{ready" + boundary + "}")

// ErrTimeout is returned by Command when ExifTool does not answer within the
// timeout set by SetTimeout. The ExifTool process is restarted.
var ErrTimeout = errors.New("exiftool: command timed out")

// Server wraps an instance of ExifTool that can process multiple commands sequentially.
// Servers avoid the overhead of loading ExifTool for each command.
// Servers are safe for concurrent use by multiple goroutines.
type Server struct {
	exec      string
	args      []string
	srvMtx    sync.Mutex
	cmdMtx    sync.Mutex
	done      bool
	cmd       *exec.Cmd
	stdin     printer
	stdout    *bufio.Scanner
	stderr    *bufio.Scanner
	splitFunc bufio.SplitFunc
	chout     chan<- string
	timeout   time.Duration
}

func (server *Server) isCustomSplit() bool {
	return server.splitFunc != nil
}

func newServerInternal(chout chan<- string, splitFunc bufio.SplitFunc, commonArg ...string) (*Server, error) {
	e := &Server{exec: Exec}

	if splitFunc != nil {
		e.splitFunc = splitFunc
	}

	if chout != nil {
		e.chout = chout
	}

	if Arg1 != "" {
		e.args = append(e.args, Arg1)
	}
	if Config != "" {
		e.args = append(e.args, "-config", Config)
	}

	e.args = append(e.args, "-stay_open", "true", "-@", "-", "-common_args", "-echo4", "{ready"+boundary+"}", "-charset", "filename=utf8")
	e.args = append(e.args, commonArg...)

	if err := e.start(); err != nil {
		return nil, err
	}
	return e, nil
}

// NewServer loads a new instance of ExifTool.
func NewServer(commonArg ...string) (*Server, error) {
	return newServerInternal(nil, nil, commonArg...)
}

// NewServerCh loads a new instance of ExifTool with output to channel file by file.
func NewServerCh(chout chan<- string, splitFunc bufio.SplitFunc, commonArg ...string) (*Server, error) {
	return newServerInternal(chout, splitFunc, commonArg...)
}

func (e *Server) start() error {
	// ExifTool may start under a watchdog shell, which starts even when ExifTool
	// cannot: a missing executable is found here
	if _, err := exec.LookPath(e.exec); err != nil {
		return err
	}
	cmd := command(e.exec, e.args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	e.stdin = printer{w: stdin}
	e.stdout = bufio.NewScanner(stdout)
	e.stderr = bufio.NewScanner(stderr)

	if e.isCustomSplit() {
		e.stdout.Split(e.splitFunc)
	} else {
		e.stdout.Split(splitReadyToken)
	}

	//e.stderr.Split(splitReadyToken) //don't need to change stderr splitter

	err = cmd.Start()
	if err != nil {
		return err
	}

	e.cmd = cmd
	return nil
}

// SetTimeout limits how long a single Command may take; 0 (the default) means no
// limit. On timeout the ExifTool process is killed and restarted and Command
// returns ErrTimeout, so one broken file cannot block the server forever.
func (e *Server) SetTimeout(d time.Duration) {
	e.cmdMtx.Lock()
	defer e.cmdMtx.Unlock()
	e.timeout = d
}

// restart kills the ExifTool process and starts a new one.
func (e *Server) restart() error {
	e.srvMtx.Lock()
	defer e.srvMtx.Unlock()
	if e.done {
		return errors.New("exiftool: server is closed")
	}

	e.kill()
	return e.start()
}

// kill stops the process and reaps it, so no zombie process is left behind.
func (e *Server) kill() error {
	err := e.cmd.Process.Kill()
	_ = e.cmd.Wait() // "signal: killed" is expected here
	return err
}

// Close causes ExifTool to exit immediately and waits until the process is reaped.
func (e *Server) Close() error {
	e.srvMtx.Lock()
	defer e.srvMtx.Unlock()

	if e.done {
		return nil
	}

	err := e.kill()
	e.done = true
	return err
}

// Shutdown gracefully shuts down ExifTool without interrupting any commands,
// and waits for it to complete.
func (e *Server) Shutdown() error {
	e.cmdMtx.Lock()
	defer e.cmdMtx.Unlock()

	e.stdin.print("-stay_open", "false")
	e.stdin.close()

	err := e.cmd.Wait()
	return err
}

// Command runs an ExifTool command with the given arguments and returns its stdout.
// Commands should neither read from stdin, nor write binary data to stdout.
func (e *Server) Command(arg ...string) ([]byte, error) {
	if e.isCustomSplit() {
		return nil, errors.New("err exiftool: Shouldn't use regular Command with custom splitFunc\n Use CommandCh instead")
	}

	e.cmdMtx.Lock()
	defer e.cmdMtx.Unlock()

	e.stdin.print(arg...)
	err := e.stdin.print("-execute" + boundary)
	if err != nil {
		return nil, e.restartAfter(err)
	}

	r := e.awaitResult()
	if r.err != nil {
		return nil, e.restartAfter(r.err)
	}

	// ExifTool reports problems with a file on stderr but still prints what it could
	// read: return that output together with the error instead of dropping it.
	if len(r.stderr) > 0 {
		if errmsg := string(bytes.TrimSpace(r.stderr)); errmsg != string(endPattern) {
			return r.stdout, errors.New("exiftool: " + errmsg)
		}
	}
	return r.stdout, nil
}

// restartAfter restarts ExifTool after a failed command and returns the cause,
// joined with the restart error if the new process could not be started.
func (e *Server) restartAfter(cause error) error {
	if err := e.restart(); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

type result struct {
	stdout []byte
	stderr []byte
	err    error
}

// awaitResult reads one command result, giving up after e.timeout (if set).
func (e *Server) awaitResult() result {
	// Capture the scanners: a restart after a timeout replaces them while the
	// abandoned read is still returning from the killed process.
	stdout, stderr := e.stdout, e.stderr
	if e.timeout <= 0 {
		return scanResult(stdout, stderr)
	}

	ch := make(chan result, 1)
	go func() { ch <- scanResult(stdout, stderr) }()

	timer := time.NewTimer(e.timeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r
	case <-timer.C:
		return result{err: ErrTimeout}
	}
}

func scanResult(stdout, stderr *bufio.Scanner) result {
	if !stdout.Scan() {
		return result{err: scanErr(stdout)}
	}
	out := append([]byte(nil), stdout.Bytes()...)
	if !stderr.Scan() {
		return result{err: scanErr(stderr)}
	}
	return result{stdout: out, stderr: append([]byte(nil), stderr.Bytes()...)}
}

func scanErr(s *bufio.Scanner) error {
	if err := s.Err(); err != nil {
		return err
	}
	return io.EOF
}

// Command runs an ExifTool command with the given arguments and put its stdout to channel.
// Commands should neither read from stdin, nor write binary data to stdout.
func (e *Server) CommandCh(arg ...string) error {
	if !e.isCustomSplit() {
		return errors.New("err exiftool: for default splitter 'by command' better to use regular Command")
	}

	e.cmdMtx.Lock()
	defer e.cmdMtx.Unlock()

	e.stdin.print(arg...)
	err := e.stdin.print("-execute" + boundary)
	if err != nil {
		return e.restartAfter(err)
	}

	for e.stdout.Scan() {
		e.chout <- e.stdout.Text()
	}

	if err := e.stdout.Err(); err != nil {
		e.chout <- "err exiftool stdout: " + err.Error()
		return e.restartAfter(err)
	}

	for e.stderr.Scan() {
		msg := e.stderr.Text()
		if msg == string(endPattern) {
			break
		}
		if msg != "" {
			e.chout <- "err exiftool stderr: " + msg
		}
	}

	if err := e.stderr.Err(); err != nil {
		e.chout <- "err exiftool stderr: " + err.Error()
		return e.restartAfter(err)
	}

	return nil
}
