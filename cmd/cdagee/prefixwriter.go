package main

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

// prefixWriter is an io.Writer that prefixes each line with "[prefix] ".
// Incomplete lines are buffered until a newline arrives. A shared mutex
// ensures that output from concurrent targets does not interleave mid-line.
type prefixWriter struct {
	prefix string
	dest   io.Writer
	mu     *sync.Mutex
	buf    bytes.Buffer
}

func newPrefixWriter(prefix string, dest io.Writer, mu *sync.Mutex) *prefixWriter {
	return &prefixWriter{
		prefix: prefix,
		dest:   dest,
		mu:     mu,
	}
}

// Write buffers input and emits complete lines with the prefix. Each complete
// line is written atomically under the shared mutex.
func (pw *prefixWriter) Write(p []byte) (int, error) {
	pw.buf.Write(p)

	for {
		line, err := pw.buf.ReadBytes('\n')
		if err != nil {
			// No more complete lines; put the partial back.
			pw.buf.Write(line)
			break
		}
		// line includes the trailing '\n'
		if writeErr := pw.writeLine(line[:len(line)-1]); writeErr != nil {
			return len(p), writeErr
		}
	}

	return len(p), nil
}

// Flush writes any remaining buffered content (a trailing partial line with
// no final newline). This is a no-op if the buffer is empty.
func (pw *prefixWriter) Flush() error {
	if pw.buf.Len() == 0 {
		return nil
	}
	remaining := pw.buf.Bytes()
	pw.buf.Reset()
	return pw.writeLine(remaining)
}

func (pw *prefixWriter) writeLine(line []byte) error {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	_, err := fmt.Fprintf(pw.dest, "[%s] %s\n", pw.prefix, line)
	return err
}
