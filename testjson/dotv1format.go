package testjson

import (
	"bufio"
	"io"
)

func dotsFormatV1(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)
	// nolint:errcheck
	return eventFormatterFunc(func(event TestEvent, exec *Execution) error {
		pkg := exec.Package(event.Package)
		switch {
		case event.PackageEvent():
			return nil
		case event.Action == ActionRun && pkg.Total == 1:
			buf.WriteString("[" + RelativePackagePath(event.Package) + "]")
			return buf.Flush()
		}
		buf.WriteString(fmtDot(event))
		return buf.Flush()
	})
}

func fmtDot(event TestEvent) string {
	withColor := colorEvent(event)
	switch event.Action {
	case ActionPass:
		return withColor("·")
	case ActionFail:
		return withColor("✖")
	case ActionSkip:
		return withColor("↷")
	}
	return ""
}
