//go:build !windows
// +build !windows

package dotwriter

import (
	"fmt"
	"strings"
)

// clear the line and move the cursor up
var clear = fmt.Sprintf("%c[%dA%c[2K", ESC, 1, ESC)
var hide = fmt.Sprintf("%c[?25l", ESC)
var show = fmt.Sprintf("%c[?25h", ESC)
var startSeq = "\x1BP=1s\x1B\x5c"
var endSeq = "\x1BP=2s\x1B\x5c"

func (w *Writer) clearLines(count int) {
	_, _ = fmt.Fprint(w.out, strings.Repeat(clear, count))
}
func (w *Writer) up(count int) {
	_, _ = fmt.Fprint(w.out, fmt.Sprintf("%c[%dA", ESC, count))
}
func (w *Writer) down() {
	_, _ = fmt.Fprint(w.out, fmt.Sprintf("%c[1E", ESC))
}
func (w *Writer) right() {
	_, _ = fmt.Fprint(w.out, fmt.Sprintf("%c[1C", ESC))
}
func (w *Writer) clearRest() {
	_, _ = fmt.Fprint(w.out, fmt.Sprintf("%c[0K", ESC))
}

// hideCursor hides the cursor and returns a function to restore the cursor back.
func (w *Writer) hideCursor() {
	//_, _ = fmt.Fprint(w.out, startSeq)
	_, _ = fmt.Fprint(w.out, hide)
}

func (w *Writer) showCursor() {
	_, _ = fmt.Fprint(w.out, show)
	//_, _ = fmt.Fprint(w.out, endSeq)
}
