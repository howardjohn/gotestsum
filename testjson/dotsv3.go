package testjson

import (
	"bufio"
	"fmt"
	"golang.org/x/term"
	"gotest.tools/gotestsum/internal/dotwriter"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type pkgInfo struct {
	lastUpdate time.Time
	event      TestEvent
	start      time.Time
	events     int
}

type dotv3Formatter struct {
	pkgs       map[string]*pkgInfo
	order      []string
	writer     *dotwriter.Writer
	opts       FormatOptions
	termWidth  int
	termHeight int
	lastExec   *Execution
	last       int
	workers    map[uint64]*worker
	mu         sync.RWMutex
	e          events
}

type events struct {
	vets   int
	builds int
	links  int
	tests  int
}

type worker struct {
	events int
	name   string
	last   TraceEvent
	start  time.Time
	pkg    string
	types  string
}

func newDotv3Formatter(out io.Writer, opts FormatOptions) EventFormatter {
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	f := &dotv3Formatter{
		pkgs:       make(map[string]*pkgInfo),
		workers:    map[uint64]*worker{},
		termWidth:  w,
		termHeight: h,
		opts:       opts,
	}
	go func() {
		t := time.NewTicker(time.Millisecond * 200)
		for {
			for {
				select {
				case <-t.C:
					f.emit(out)
				}
			}
		}
	}()
	return f
}

func (d *dotv3Formatter) FormatTrace(event TraceEvent, exec *Execution) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, f := d.workers[event.TID]; !f {
		d.workers[event.TID] = &worker{}
	}
	d.workers[event.TID].events++
	name := event.Name
	r, _, ok := strings.Cut(name, " -> ")
	if ok {
		name = r
	}
	//et := time.Unix(0, int64(event.Time)*int64(time.Microsecond))
	//fmt.Printf("behind %v: %v %v\n", time.Since(et), event.Name, event.TID)

	if d.workers[event.TID].last.Time > event.Time {
		// Stale event, ignore
		return nil
	}

	d.workers[event.TID].name = name
	en := name[len("Executing action (") : len(name)-1]
	splits := strings.Split(en, " ")
	pkg := splits[len(splits)-1]
	types := strings.Join(splits[0:len(splits)-1], " ")
	switch types {
	case "link":
		d.e.links++
	case "build":
		d.e.builds++
	case "test run":
		d.e.tests++
	case "vet":
		d.e.vets++
	}
	switch event.Phase {
	//case "B", "s":
	case "B":
		//d.workers[event.TID].events++
		d.workers[event.TID].pkg = pkg
		d.workers[event.TID].types = types
		d.workers[event.TID].last = event
		d.workers[event.TID].start = time.Unix(0, int64(event.Time)*int64(time.Microsecond))
	//case "E", "f":
	case "E":
		d.workers[event.TID].last = TraceEvent{}
		d.workers[event.TID].start = time.Time{}
	default:
		//panic(fmt.Sprintf("unknown event %+v", event))
	}
	return nil
}

func (d *dotv3Formatter) Format(event TestEvent, exec *Execution) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pkgs[event.Package] == nil {
		d.pkgs[event.Package] = &pkgInfo{
			start: time.Now(),
		}
		d.order = append(d.order, event.Package)
	}
	line := d.pkgs[event.Package]
	line.lastUpdate = event.Time
	line.event = event
	line.events++

	d.lastExec = exec

	switch event.Action {
	case ActionOutput, ActionBench:
		return nil
	}

	sort.Slice(d.order, d.orderByLastUpdated)

	return nil
}

// orderByLastUpdated so that the most recently updated packages move to the
// bottom of the list, leaving completed package in the same order at the top.
func (d *dotv3Formatter) orderByLastUpdated(i, j int) bool {
	return d.pkgs[d.order[i]].lastUpdate.Before(d.pkgs[d.order[j]].lastUpdate)
}

func (d *dotv3Formatter) emit(out io.Writer) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	bw := bufio.NewWriter(out)
	const ESC = 27
	var clear = fmt.Sprintf("%c[%dA%c[2K", ESC, 1, ESC)
	var hide = fmt.Sprintf("%c[?25l", ESC)
	var show = fmt.Sprintf("%c[?25h", ESC)
	keys := []string{}
	for k := range d.pkgs {
		keys = append(keys, k)
	}
	//var event TestEvent
	//if d.pkgs["gotest.tools/gotestsum/cmd/tool/slowest"] != nil {
	//	event = d.pkgs["gotest.tools/gotestsum/cmd/tool/slowest"].event
	//}
	// Clear
	_, _ = fmt.Fprint(bw, strings.Repeat(clear, d.last))
	lines := 0

	_, _ = fmt.Fprint(bw, hide)
	write := func(s string) {
		if len(s) > d.termWidth {
			s = s[:d.termWidth]
		}
		bw.WriteString(s)
		bw.WriteString("\n")
		lines++
	}
	for i := uint64(0); i < uint64(len(d.workers)); i++ {
		if _, f := d.workers[i]; f {
			e := d.workers[i]
			events := 0
			if p, f := d.pkgs[e.pkg]; f {
				events = p.events
			}
			write(fmt.Sprintf(
				"Worker %d: %v for %v:\t%v %v\t(%v)",
				i,
				e.events,
				time.Since(e.start).Truncate(time.Millisecond),
				events,
				e.types,
				e.pkg,
			))
		} else {
			write(fmt.Sprintf("Worker %d: ?", i))
		}
	}

	write(strings.Join(d.order, " "))
	//for _, k := range d.order {
	//	v := d.pkgs[k]
	//	pkg := d.lastExec.Package(k)
	//	name := RelativePackagePath(k)
	//	status := "✅"
	//	if !v.event.Action.IsTerminal() {
	//		status = fmtDotElapsed(pkg)
	//	}
	//	write(fmt.Sprintf("%v: %v %v", name, v.events, status))
	//}
	ag := d.opts.ActionGraph
	e := events{}
	for _, ge := range ag {
		switch ge.Mode {
		case "vet":
			e.vets++
		case "build":
			e.builds++
		case "link":
			e.links++
		case "test run":
			e.tests++
		}
	}
	g := d.e
	write(fmt.Sprintf("WANT: %d vets, %d builds, %d tests, %d links", e.vets, e.builds, e.tests, e.links))
	write(fmt.Sprintf("HAVE: %d vets, %d builds, %d tests, %d links", g.vets, g.builds, g.tests, g.links))
	d.last = lines
	_, _ = fmt.Fprint(bw, show)
	bw.Flush()
}
