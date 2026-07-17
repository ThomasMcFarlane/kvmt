package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintLastLines(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := printLastLines(&output, strings.NewReader("one\ntwo\nthree\nfour\nfive\n"), 3); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "three\nfour\nfive\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrintLastLinesShowsEmptyLog(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := printLastLines(&output, strings.NewReader(""), 40); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "The service log is empty.\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
