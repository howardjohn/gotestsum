/*
Package dotwriter implements a buffered Writer for updating progress on the
terminal.
*/
package dotwriter

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
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

var i = 0
var iter = 0

// Flush the buffer, writing all buffered lines to out
func (w *Writer) Flush() error {
	if w.buf.Len() == 0 {
		return nil
	}
	iter++

	last := w.last

	b := w.buf.Bytes()
	w.last = bytes.Clone(b)
	acts := &bytes.Buffer{}
	os.WriteFile(fmt.Sprintf("/tmp/outs/old-%d.txt", iter), last, 0644)
	os.WriteFile(fmt.Sprintf("/tmp/outs/new-%d.txt", iter), w.last, 0644)

	oldLine := w.lineCount
	w.lineCount = bytes.Count(b, []byte{'\n'})
	w.hideCursor()
	w.up(oldLine)

	writes := 0
	lines := bytes.Split(b, []byte{'\n'})
	acts.WriteString(fmt.Sprintf("Raw: [%v]", string(b)))
	for i, line := range lines {
		w.out.Write(line)
		writes++
		w.clearRest()
		if i != len(lines)-1 {
			//w.out.Write([]byte(fmt.Sprint(iter)))
			w.out.Write([]byte{'\n'})
			acts.WriteString(fmt.Sprintf("write with newline: up=%d l=%d [%v]\n", oldLine, len(b), string(line)))
		} else {
			acts.WriteString(fmt.Sprintf("write w/o newline: up=%d l=%d [%v]\n", oldLine, len(b), string(line)))

		}
	}
	//w.down()
	//w.down()
	//w.down()
	//w.down()
	w.showCursor()
	//w.out.Write([]byte(fmt.Sprintf("[%d %d %d %d]", i, len(old), len(now), w.lineCount)))
	os.WriteFile(fmt.Sprintf("/tmp/outs/acts-%d.txt", iter), acts.Bytes(), 0644)
	w.buf.Reset()
	//w.out.Write([]byte(fmt.Sprintf(
	//	"[%d %d %d]", len(b), oldLine, writes,
	//)))
	//w.out.Write(b)
	w.out.(*bufio.Writer).Flush()
	//time.Sleep(time.Second * 1)
	return nil
	var err error
	if oldLine != w.lineCount {
		//	// Full reset
		//	// TODO: we can do incremental here as well, we just need to be careful
		w.clearLines(oldLine)
		_, err = w.out.Write(b)
	} else {
		i++
		// Incremental reset
		w.up(w.lineCount)
		old := bytes.Split(last, []byte{'\n'})
		now := bytes.Split(w.last, []byte{'\n'})
		for i := range now {
			// Already verified these are match
			var ol []byte
			if len(old) > i {
				ol = old[i]
			}
			nl := now[i]
			if bytes.Equal(ol, nl) {
				//w.out.Write([]byte(fmt.Sprint("skip ", i+1)))
				w.down()
			} else {
				//w.out.Write([]byte(fmt.Sprint(i + 1)))
				w.out.Write(nl)
				//w.clearRest()
				w.out.Write([]byte{'\n'})
				//w.down()
			}
		}
	}
	// Now move to next line
	//w.down()
	//}
	//}

	w.showCursor()
	//w.out.Write([]byte(fmt.Sprintf("[%d %d %d %d]", i, len(old), len(now), w.lineCount)))
	w.buf.Reset()
	w.out.(*bufio.Writer).Flush()
	return err
}

// Write saves buf to a buffer
func (w *Writer) Write(buf []byte) (int, error) {
	return w.buf.Write(buf)
}
