/*
Package dotwriter implements a buffered Writer for updating progress on the
terminal.
*/
package dotwriter

import (
	"bufio"
	"bytes"
	"io"
	"time"
)

// ESC is the ASCII code for escape character
const ESC = 27

// Writer buffers writes until Flush is called. Flush clears previously written
// lines before writing new lines from the buffer.
type Writer struct {
	out       io.Writer
	buf       bytes.Buffer
	last      []byte
	lineCount int
	t         *time.Timer
}

// New returns a new Writer
func New(out io.Writer) *Writer {
	out = bufio.NewWriter(out)
	w := &Writer{out: out}
	return w
}

// Flush the buffer, writing all buffered lines to out
func (w *Writer) Flush() error {
	if w.buf.Len() == 0 {
		return nil
	}
	w.hideCursor()
	w.up(w.lineCount)

	lines := bytes.Split(w.buf.Bytes(), []byte{'\n'})
	w.lineCount = len(lines) - 1
	for i, line := range lines {
		w.out.Write(line)
		w.clearRest()
		if i != len(lines)-1 {
			w.out.Write([]byte{'\n'})
		} else {

		}
	}
	w.showCursor()
	w.buf.Reset()
	w.out.(*bufio.Writer).Flush()
	return nil
}

// Write saves buf to a buffer
func (w *Writer) Write(buf []byte) (int, error) {
	return w.buf.Write(buf)
}
