package exiftool

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os/exec"
	"sync"
)

const boundary = "1854673209"

var endPattern = []byte("{ready" + boundary + "}")

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
	cmd := exec.Command(e.exec, e.args...)

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

func (e *Server) restart() {
	e.srvMtx.Lock()
	defer e.srvMtx.Unlock()
	if e.done {
		return
	}

	e.cmd.Process.Kill()
	e.cmd.Process.Release()
	e.start()
}

// Close causes ExifTool to exit immediately.
// Close does not wait until ExifTool has actually exited.
func (e *Server) Close() error {
	e.srvMtx.Lock()
	defer e.srvMtx.Unlock()

	if e.done {
		return nil
	}

	err := e.cmd.Process.Kill()
	e.cmd.Process.Release()
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
		e.restart()
		return nil, err
	}

	if !e.stdout.Scan() {
		err := e.stdout.Err()
		if err == nil {
			err = io.EOF
		}
		e.restart()
		return nil, err
	}
	if !e.stderr.Scan() {
		err := e.stderr.Err()
		if err == nil {
			err = io.EOF
		}
		e.restart()
		return nil, err
	}

	if len(e.stderr.Bytes()) > 0 {
		if errmsg := string(bytes.TrimSpace(e.stderr.Bytes())); errmsg != string(endPattern) {
			return nil, errors.New("exiftool: " + errmsg)
		}
	}
	return append([]byte(nil), e.stdout.Bytes()...), nil
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
		e.restart()
		return err
	}

	for e.stdout.Scan() {
		e.chout <- e.stdout.Text()
	}

	if err := e.stdout.Err(); err != nil {
		e.chout <- "err exiftool stdout: " + err.Error()
		e.restart()
		return err
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
		e.restart()
		return err
	}

	return nil
}
