package main

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestPrefixWriterSingleLine(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	pw := newPrefixWriter("target-a", &out, &mu)

	fmt.Fprint(pw, "hello world\n")

	if got := out.String(); got != "[target-a] hello world\n" {
		t.Errorf("got %q, want %q", got, "[target-a] hello world\n")
	}
}

func TestPrefixWriterMultiLine(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	pw := newPrefixWriter("tgt", &out, &mu)

	fmt.Fprint(pw, "line1\nline2\nline3\n")

	want := "[tgt] line1\n[tgt] line2\n[tgt] line3\n"
	if got := out.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrefixWriterPartialLines(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	pw := newPrefixWriter("x", &out, &mu)

	fmt.Fprint(pw, "hel")
	if out.Len() != 0 {
		t.Errorf("expected no output yet, got %q", out.String())
	}

	fmt.Fprint(pw, "lo\nwor")
	if got := out.String(); got != "[x] hello\n" {
		t.Errorf("after second write: got %q, want %q", got, "[x] hello\n")
	}

	fmt.Fprint(pw, "ld\n")
	want := "[x] hello\n[x] world\n"
	if got := out.String(); got != want {
		t.Errorf("after third write: got %q, want %q", got, want)
	}
}

func TestPrefixWriterFlush(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	pw := newPrefixWriter("f", &out, &mu)

	fmt.Fprint(pw, "trailing")
	if out.Len() != 0 {
		t.Errorf("expected no output before flush, got %q", out.String())
	}

	if err := pw.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[f] trailing\n" {
		t.Errorf("after flush: got %q, want %q", got, "[f] trailing\n")
	}
}

func TestPrefixWriterFlushEmpty(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	pw := newPrefixWriter("e", &out, &mu)

	if err := pw.Flush(); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for empty flush, got %q", out.String())
	}
}

func TestPrefixWriterConcurrent(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex

	pw1 := newPrefixWriter("a", &out, &mu)
	pw2 := newPrefixWriter("b", &out, &mu)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(2)

	write := func(pw *prefixWriter, prefix string) {
		defer wg.Done()
		for i := 0; i < n; i++ {
			fmt.Fprintf(pw, "line %d\n", i)
		}
	}

	go write(pw1, "a")
	go write(pw2, "b")
	wg.Wait()

	lines := bytes.Split(bytes.TrimRight(out.Bytes(), "\n"), []byte("\n"))
	if len(lines) != 2*n {
		t.Fatalf("expected %d lines, got %d", 2*n, len(lines))
	}

	// Every line must start with "[a] " or "[b] " — no interleaving.
	for i, line := range lines {
		s := string(line)
		if !bytes.HasPrefix(line, []byte("[a] ")) && !bytes.HasPrefix(line, []byte("[b] ")) {
			t.Errorf("line %d: unexpected prefix: %q", i, s)
		}
	}
}
