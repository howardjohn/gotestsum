/*
Package dotwriter implements a buffered Writer for updating progress on the
terminal.
*/
package dotwriter

import (
	"bytes"
	"io"
)

// Writer buffers writes until Flush is called. Flush clears previously written
// lines before writing new lines from the buffer.
// The main logic is platform specific, see the related files.
type Writer struct {
	out       io.Writer
	buf       bytes.Buffer
	lineCount int
	h         int
}

// New returns a new Writer
func New(out io.Writer, h int) *Writer {
	return &Writer{out: out, h: h - 2}
}

// Write saves buf to a buffer
func (w *Writer) Write(buf []byte) (int, error) {
	return w.buf.Write(buf)
}
