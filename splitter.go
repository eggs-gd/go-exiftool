package exiftool

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strings"
)

func splitReadyToken(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.Index(data, []byte(endPattern)); i >= 0 {
		if n := bytes.IndexByte(data[i:], '\n'); n >= 0 {
			if atEOF && len(data) == (n+i+1) { // nothing left to scan
				return n + i + 1, data[:i], bufio.ErrFinalToken
			} else {
				return n + i + 1, data[:i], nil
			}
		}
	}

	if atEOF {
		return len(data), data, io.EOF
	}
	return 0, nil, nil
}

func DefaultSplitter(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if endPos := bytes.Index(data, endPattern); endPos >= 0 {
		adv := endPos + len(endPattern)
		tk := data[:endPos]
		if strings.TrimSpace(string(tk)) == "" { // check case when there is only \n\n etc
			tk = nil
		}
		return adv,
			tk,
			bufio.ErrFinalToken
	}

	startObject := []byte("======== ")
	startReport := regexp.MustCompile(`\n+\s+`)

	// Object start
	if i := bytes.Index(data, startObject); i >= 0 {
		// Find next object
		if j := bytes.Index(data[i+len(startObject):], startObject); j > 0 {
			// If so, then return token between them
			return i + j + len(startObject),
				data[:i+j+len(startObject)],
				nil
		}

		// If there is not next object try to check exiftool report (usually started from empty spaces)
		if reportIndices := startReport.FindIndex(data[i+len(startObject):]); reportIndices != nil {
			// Then return token between start pattern and report
			return reportIndices[0] + len(startObject),
				data[:reportIndices[0]+len(startObject)],
				nil
		}
	}

	if atEOF {
		return len(data), data, io.EOF
	}

	return 0, nil, nil
}
