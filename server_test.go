package exiftool

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestServer(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	defer e.Close()

	// ask for version number
	out, err := e.Command("-ver")
	if err != nil {
		t.Error(err)
	} else if ver, err := strconv.ParseFloat(string(bytes.TrimSpace(out)), 64); err != nil {
		t.Error(err)
	} else {
		t.Log(ver)
	}

	// shutdown the server
	err = e.Shutdown()
	if err != nil {
		t.Error(err)
	}

	// shutdown should not be called twice
	err = e.Shutdown()
	if err == nil {
		t.Error("repeated shutdown")
	}

	// commands should fail now
	_, err = e.Command("-ver")
	if err == nil {
		t.Error("command after shutdown")
	}

	// close should be fine at any time
	err = e.Close()
	if err != nil {
		t.Error(err)
	}
}

func TestServerCh(t *testing.T) {
	ch := make(chan string, 100) // need buffered channel to not block flow with sync run
	e, err := NewServerCh(ch, DefaultSplitter)

	if err != nil {
		t.Fatal(err)
	}

	defer e.Close()

	// shouldn't work with the regular command
	out, err := e.Command("-ver")
	if err != nil {
		t.Log(out, err)
	} else {
		t.Error(err)
	}

	// ask for version number

	err = e.CommandCh("-ver")

	if err != nil {
		t.Error(err)
	} else if ver, err := strconv.ParseFloat(strings.TrimSpace(<-ch), 64); err != nil {
		t.Error(err)
	} else {
		t.Log(ver)
	}

	// close should be fine at any time
	err = e.Close()
	if err != nil {
		t.Error(err)
	}

	go func() {
		// channel should be closed
		msg, ok := <-ch
		if ok {
			t.Error(errors.New("channel still not closed, msg: " + msg))
		}
	}()

	close(ch)
}
