package exiftool

import (
	"bufio"
	"strings"
	"testing"
)

type testcase struct {
	name           string
	input          []byte
	expectedTokens []string
	expectedErr    error
}

func TestDefaultSplitter(t *testing.T) {
	tests := []testcase{
		{
			name:  "Two objects and report",
			input: []byte("======== ./_MG_5112.JPG\nMIME Type : image/jpeg\nSomeOtherParam : image/jpeg\nOne Other : image/jpeg\n======== ./_MG_5113.JPG\nMIME Type : image/jpeg\n    3 image files read\n" + string(endPattern)),
			expectedTokens: []string{
				"======== ./_MG_5112.JPG\nMIME Type : image/jpeg\nSomeOtherParam : image/jpeg\nOne Other : image/jpeg",
				"======== ./_MG_5113.JPG\nMIME Type : image/jpeg",
				"    3 image files read",
			},
			expectedErr: bufio.ErrFinalToken,
		},
		{
			name:           "Single object without report",
			input:          []byte("======== ./_MG_5112.JPG\nMIME Type : image/jpeg\n" + string(endPattern)),
			expectedTokens: []string{"======== ./_MG_5112.JPG\nMIME Type : image/jpeg"},
			expectedErr:    bufio.ErrFinalToken,
		},
		{
			name:  "Only report after object",
			input: []byte("======== ./_MG_5112.JPG\nMIME Type : image/jpeg\n    1 image file read\n" + string(endPattern)),
			expectedTokens: []string{
				"======== ./_MG_5112.JPG\nMIME Type : image/jpeg",
				"    1 image file read",
			},
			expectedErr: bufio.ErrFinalToken,
		},
		{
			name:           "Only report",
			input:          []byte("    1 image file read\n" + string(endPattern)),
			expectedTokens: []string{"    1 image file read"},
			expectedErr:    bufio.ErrFinalToken,
		},
		{
			name:           "No objects or reports",
			input:          []byte("random data" + string(endPattern)),
			expectedTokens: []string{"random data"},
			expectedErr:    bufio.ErrFinalToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSplitter(t, DefaultSplitter, tt)
		})
	}
}

func runSplitter(t *testing.T, splitter bufio.SplitFunc, tt testcase) {
	var tokens []string

	var advance int
	var token []byte
	var err error

	start := 0
	for i := range tt.input {
		advance, token, err = splitter(tt.input[start:i+1], i == len(tt.input)-1)
		if token != nil {
			tokens = append(tokens, string(token))
			start += advance
		}
		if err != nil {
			break
		}
	}

	if len(tokens) != len(tt.expectedTokens) {
		t.Errorf("expected %d tokens, got %d", len(tt.expectedTokens), len(tokens))
		for i, tc := range tokens {
			t.Logf("got %d ->\n%v\n", i, tc)
		}
	}

	for i, expected := range tt.expectedTokens {
		if strings.TrimSpace(tokens[i]) != strings.TrimSpace(expected) {
			t.Errorf("token %d:\nexpected -> \n%v\ngot -> \n%v", i, expected, tokens[i])
		}
	}

	if err != tt.expectedErr {
		t.Errorf("expected error: %v, got: %v", tt.expectedErr, err)
	}

}
