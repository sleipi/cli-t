package display

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	StatusRun = "RUN"
	StatusOK  = "OK"
	StatusERR = "ERR"
)

// EntryInfo holds the result of a single entry execution for display purposes.
type EntryInfo struct {
	Command     string
	Passed      bool
	Skipped     bool
	SkipReason  string
	ExitCode    int
	AssertCount int
	Failures    []string
	Stdout      string
	Stderr      string
}

// --- ProgressDisplay (unified display for compact and verbose modes) ---

type fileState struct {
	name      string
	completed int
	total     int
	status    string // StatusRun, StatusOK, StatusERR
	startTime time.Time
	endTime   time.Time
	hidden    bool
}

// ProgressDisplay shows running files as dynamic progress bars at the bottom,
// and emits finished file output (compact or verbose) as permanent lines above.
type ProgressDisplay struct {
	mu                sync.Mutex
	w                 io.Writer
	files             []fileState
	dynamic           bool // true = TTY (use ANSI cursor movement), false = static
	maxDynamic        int  // max lines in dynamic block (min(parallel, 16))
	printed           bool // whether we've rendered the dynamic block
	lastRenderedLines int  // lines in last dynamic render
}

// NewProgressDisplay creates a ProgressDisplay.
// dynamic=true uses ANSI escape sequences for in-place updates.
// dynamic=false prints output only when files finish.
// maxDynamic caps the number of running-file lines shown (use min(parallel, 16)).
func NewProgressDisplay(w io.Writer, dynamic bool, maxDynamic int) *ProgressDisplay {
	if maxDynamic <= 0 {
		maxDynamic = 8
	}
	if maxDynamic > 16 {
		maxDynamic = 16
	}
	return &ProgressDisplay{w: w, dynamic: dynamic, maxDynamic: maxDynamic}
}

func (d *ProgressDisplay) Start(files []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files = make([]fileState, len(files))
	for i, f := range files {
		d.files[i] = fileState{name: filepath.Base(f), status: StatusRun, startTime: time.Now()}
	}
}

func (d *ProgressDisplay) UpdateProgress(fileIdx, completed, total int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files[fileIdx].completed = completed
	d.files[fileIdx].total = total

	if d.dynamic {
		d.renderDynamic()
	}
}

// FinishFile marks a file as complete and prints its permanent output.
// If output is empty, the compact status line is auto-generated.
func (d *ProgressDisplay) FinishFile(fileIdx int, passed bool, output string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files[fileIdx].completed = d.files[fileIdx].total
	d.files[fileIdx].endTime = time.Now()
	if passed {
		d.files[fileIdx].status = StatusOK
	} else {
		d.files[fileIdx].status = StatusERR
	}

	if output == "" {
		output = d.formatFileLine(fileIdx)
	}

	if d.dynamic {
		d.clearDynamic()
		fmt.Fprint(d.w, output)
		d.renderDynamic()
	} else {
		fmt.Fprint(d.w, output)
	}
}

func (d *ProgressDisplay) HideFile(fileIdx int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files[fileIdx].hidden = true
	d.files[fileIdx].status = StatusOK // don't show in dynamic block
}

func (d *ProgressDisplay) FileError(fileIdx int, output string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files[fileIdx].status = StatusERR
	d.files[fileIdx].endTime = time.Now()

	if d.dynamic {
		d.clearDynamic()
		if output != "" {
			fmt.Fprint(d.w, output)
		}
		d.renderDynamic()
	} else {
		if output != "" {
			fmt.Fprint(d.w, output)
		}
	}
}

func (d *ProgressDisplay) Finish() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.dynamic {
		d.clearDynamic()
		fmt.Fprintln(d.w)
	}
}

// renderDynamic shows up to maxDynamic running files sorted by longest-running.
func (d *ProgressDisplay) renderDynamic() {
	// Collect running file indices sorted by duration (longest first)
	type runInfo struct {
		idx int
		dur time.Duration
	}
	var running []runInfo
	for i := range d.files {
		if d.files[i].hidden || d.files[i].status != StatusRun {
			continue
		}
		running = append(running, runInfo{i, time.Since(d.files[i].startTime)})
	}

	if len(running) == 0 {
		d.clearDynamic()
		return
	}

	// Sort by duration descending (longest first)
	for i := 0; i < len(running)-1; i++ {
		for j := i + 1; j < len(running); j++ {
			if running[j].dur > running[i].dur {
				running[i], running[j] = running[j], running[i]
			}
		}
	}

	// Cap at maxDynamic
	if len(running) > d.maxDynamic {
		running = running[:d.maxDynamic]
	}

	var buf strings.Builder

	// Overwrite previous dynamic block
	if d.printed && d.lastRenderedLines > 0 {
		fmt.Fprintf(&buf, "\033[%dA", d.lastRenderedLines)
	}

	lines := 0
	for _, r := range running {
		fmt.Fprintf(&buf, "\r\033[K")
		buf.WriteString(d.formatFileLine(r.idx))
		lines++
	}

	// Clear extra lines from previous render
	extra := d.lastRenderedLines - lines
	for i := 0; i < extra; i++ {
		fmt.Fprintf(&buf, "\r\033[K\n")
	}
	if extra > 0 {
		fmt.Fprintf(&buf, "\033[%dA", extra)
	}

	d.lastRenderedLines = lines
	d.printed = true

	fmt.Fprint(d.w, buf.String())
}

// clearDynamic removes the dynamic progress lines.
func (d *ProgressDisplay) clearDynamic() {
	if d.printed && d.lastRenderedLines > 0 {
		var buf strings.Builder
		fmt.Fprintf(&buf, "\033[%dA", d.lastRenderedLines)
		for i := 0; i < d.lastRenderedLines; i++ {
			fmt.Fprintf(&buf, "\r\033[K\n")
		}
		fmt.Fprintf(&buf, "\033[%dA", d.lastRenderedLines)
		fmt.Fprint(d.w, buf.String())
	}
	d.lastRenderedLines = 0
	d.printed = false
}

func (d *ProgressDisplay) formatFileLine(idx int) string {
	f := d.files[idx]
	done := f.status == StatusOK || f.status == StatusERR
	bar := RenderBar(f.completed, max(f.total, 1), done)

	var statusColor string
	switch f.status {
	case StatusOK:
		statusColor = ColorGreen
	case StatusERR:
		statusColor = ColorRed
	default:
		statusColor = ColorYellow
	}

	// Pad status to 3 chars
	status := f.status
	if len(status) < 3 {
		status += strings.Repeat(" ", 3-len(status))
	}

	// Timing
	var elapsed time.Duration
	if done && !f.endTime.IsZero() {
		elapsed = f.endTime.Sub(f.startTime)
	} else {
		elapsed = time.Since(f.startTime)
	}
	timeStr := FormatDuration(elapsed)

	var suffix string
	if f.total > 0 {
		if done {
			suffix = fmt.Sprintf(" (%d/%d) took %s", f.completed, f.total, timeStr)
		} else {
			suffix = fmt.Sprintf(" (%d/%d) %s", f.completed, f.total, timeStr)
		}
	}

	return fmt.Sprintf("%s%s%s %s - %s%s%s%s\n", statusColor, status, ColorReset, bar, f.name, ColorGray, suffix, ColorReset)
}

// RenderBar generates a progress bar string like [=====>    ] or [==========].
func RenderBar(completed, total int, done bool) string {
	const width = 10

	if done {
		return "[" + strings.Repeat("=", width) + "]"
	}

	var filled int
	if total > 0 {
		filled = (completed * width) / total
	}
	if filled >= width {
		filled = width - 1
	}

	bar := strings.Repeat("=", filled) + ">" + strings.Repeat(" ", width-filled-1)
	return "[" + bar + "]"
}
