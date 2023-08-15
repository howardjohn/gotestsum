package testjson

import (
	"bytes"
	"fmt"
	"golang.org/x/term"
	"gotest.tools/gotestsum/internal/dotwriter"
	"gotest.tools/gotestsum/internal/log"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type dotv3Formatter struct {
	pkgs       map[string]*dotLine
	workers    map[uint64]*worker
	order      []string
	writer     *dotwriter.Writer
	opts       FormatOptions
	termWidth  int
	termHeight int
	stop       chan struct{}
	flushed    chan struct{}
	mu         sync.RWMutex
	last       string
	summary    []byte
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
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		log.Warnf("Failed to detect terminal width for dots format, error: %v", err)
		return dotsFormatV1(out)
	}
	f := &dotv3Formatter{
		pkgs:       make(map[string]*dotLine),
		writer:     dotwriter.New(out),
		workers:    make(map[uint64]*worker),
		termWidth:  w,
		termHeight: h - 10,
		opts:       opts,
		stop:       make(chan struct{}),
		flushed:    make(chan struct{}),
	}
	go f.runWriter()
	return f
}

func (d *dotv3Formatter) Close() error {
	close(d.stop)
	<-d.flushed // Wait until we write the last data
	return nil
}

func (d *dotv3Formatter) FormatTrace(event TraceEvent, exec *Execution) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, f := d.workers[event.TID]; !f {
		d.workers[event.TID] = &worker{}
	}
	w := d.workers[event.TID]
	w.events++
	switch event.Phase {
	case "B":
		d.workers[event.TID].last = event
	case "E":
		d.workers[event.TID].last = TraceEvent{}
	default:
	}
	w.last = event
	return nil
	/*
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
	*/
}

func (d *dotv3Formatter) Format(event TestEvent, exec *Execution) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pkgs[event.Package] == nil {
		d.pkgs[event.Package] = &dotLine{builder: new(strings.Builder)}
		d.order = append(d.order, event.Package)
	}
	line := d.pkgs[event.Package]
	line.lastUpdate = event.Time

	if !event.PackageEvent() {
		line.update(fmtDot(event))
	}
	pkg := exec.Package(event.Package)

	pkgname := RelativePackagePath(event.Package) + " "
	prefix := fmtDotElapsed(pkg)
	line.checkWidth(len(prefix+pkgname), d.termWidth)
	line.checkWidth(len(prefix+pkgname), d.termWidth)
	line.out = prefix + pkgname + line.builder.String()
	line.result = pkg.Result()

	line.empty = pkg.IsEmpty()
	buf := bytes.Buffer{}
	PrintSummary(&buf, exec, SummarizeNone)
	d.summary = buf.Bytes()

	return nil
	/*
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


	*/
}
func (d *dotv3Formatter) runWriter() {
	t := time.NewTicker(time.Millisecond * 156)
	for {
		select {
		case <-d.stop:
			if err := d.write(); err != nil {
				log.Warnf("failed to write: %v", err)
			}
			close(d.flushed)
			return
		case <-t.C:
			if err := d.write(); err != nil {
				log.Warnf("failed to write: %v", err)
			}
		}
	}
}

func (d *dotv3Formatter) write() error {
	d.mu.RLock() // TODO: lock is not sufficient, we need to read from d.exec in the event handler.
	defer d.mu.RUnlock()

	// TODO summary time should update on each iteration ideally. Although that drops our "skip" optimization
	summaryLines := strings.Split(string(d.summary), "\n")

	packageLines := []*dotLine{}
	for _, pkg := range d.order {
		line := d.pkgs[pkg]
		if d.opts.HideEmptyPackages && line.empty {
			continue
		}

		packageLines = append(packageLines, line)
	}
	maxTestLines := d.termHeight - len(summaryLines)
	lines := filterLines(packageLines, maxTestLines)
	lines = append(lines, d.workerSummary())
	lines = append(lines, summaryLines...)
	res := strings.Join(lines, "\n")
	if res == d.last {
		return nil
	}
	d.last = res

	// Write empty lines for some padding
	fmt.Fprint(d.writer, "\n")
	d.writer.Write([]byte(res))

	return d.writer.Flush()
}

func (d *dotv3Formatter) workerSummary() string {
	linking := 0
	building := 0
	nothing := 0
	running := 0
	vetting := 0
	unknown := 0
	pkgs := []string{}
	workers := []string{}
	for _, w := range d.workers {
		name := w.last.Name
		if name == "" {
			nothing++
			continue
		}
		if !strings.HasPrefix(name, "Executing action (") {
			unknown++
			continue
		}
		en := name[len("Executing action (") : len(name)-1]
		splits := strings.Split(en, " ")
		pkg := splits[len(splits)-1]
		pn := strings.Split(pkg, "/")
		types := strings.Join(splits[0:len(splits)-1], " ")
		switch types {
		case "link":
			linking++
			workers = append(workers, "🔗")
		case "build":
			building++
			workers = append(workers, "🔨 ")
		case "test run":
			running++
			pkgs = append(pkgs, pn[len(pn)-1])
			workers = append(workers, "⏱️ ")
		case "vet":
			workers = append(workers, "🔍")
			vetting++
		default:
			workers = append(workers, "")
			unknown++
		}
	}
	sort.Strings(pkgs)
	sort.Strings(workers)
	return "Workers: " + strings.Join(workers, "")
	return fmt.Sprintf(" Workers: %d linking, %d building, %d running, %d vetting, %d waiting, %d unknown",
		linking, building, running, vetting, nothing, unknown)
}

// orderByLastUpdated so that the most recently updated packages move to the
// bottom of the list, leaving completed package in the same order at the top.
func (d *dotv3Formatter) orderByLastUpdated(i, j int) bool {
	return d.pkgs[d.order[i]].lastUpdate.Before(d.pkgs[d.order[j]].lastUpdate)
}

/*
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
*/
