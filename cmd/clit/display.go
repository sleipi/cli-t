package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

// --- ProgressDisplay (compact progress bars) ---

type fileState struct {
	name           string
	completed      int
	total          int
	status         string // "RUN", "OK", "ERR"
	startTime      time.Time
	endTime        time.Time
	currentComment string
}

// ProgressDisplay shows compact one-line-per-file progress bars.
type ProgressDisplay struct {
	mu                sync.Mutex
	w                 io.Writer
	files             []fileState
	dynamic           bool // true = TTY (use ANSI cursor movement), false = static
	printed           bool // whether we've printed lines (for dynamic redraws)
	lastRenderedLines int  // number of lines in last dynamic render
}

// NewProgressDisplay creates a ProgressDisplay.
// dynamic=true uses ANSI escape sequences for in-place updates.
// dynamic=false prints each file line once when it finishes.
func NewProgressDisplay(w io.Writer, dynamic bool) *ProgressDisplay {
	return &ProgressDisplay{w: w, dynamic: dynamic}
}

func (d *ProgressDisplay) Start(files []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files = make([]fileState, len(files))
	for i, f := range files {
		d.files[i] = fileState{name: filepath.Base(f), status: "RUN", startTime: time.Now()}
	}

	if d.dynamic {
		d.render()
	}
}

func (d *ProgressDisplay) UpdateProgress(fileIdx int, completed, total int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files[fileIdx].completed = completed
	d.files[fileIdx].total = total

	if d.dynamic {
		d.render()
	}
}

// UpdateEntry sets the current entry subtitle (comment or command) for a running file.
func (d *ProgressDisplay) UpdateEntry(fileIdx int, comment string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files[fileIdx].currentComment = comment

	if d.dynamic {
		d.render()
	}
}

func (d *ProgressDisplay) FinishFile(fileIdx int, passed bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files[fileIdx].completed = d.files[fileIdx].total
	d.files[fileIdx].endTime = time.Now()
	d.files[fileIdx].currentComment = ""
	if passed {
		d.files[fileIdx].status = "OK"
	} else {
		d.files[fileIdx].status = "ERR"
	}

	if d.dynamic {
		d.render()
	} else {
		// Static mode: print the line once when file finishes
		d.printFileLine(fileIdx)
	}
}

func (d *ProgressDisplay) FileError(fileIdx int, msg string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.files[fileIdx].status = "ERR"
	d.files[fileIdx].endTime = time.Now()
	if d.dynamic {
		d.render()
	} else {
		d.printFileLine(fileIdx)
	}
}

func (d *ProgressDisplay) Finish() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.dynamic {
		// Final render (all files done, no subtitles)
		d.render()
		fmt.Fprintln(d.w)
	}
}

// render redraws all file lines using ANSI cursor movement.
func (d *ProgressDisplay) render() {
	// Move cursor up to overwrite previous output
	if d.printed && d.lastRenderedLines > 0 {
		fmt.Fprintf(d.w, "\033[%dA", d.lastRenderedLines)
	}

	lines := 0
	for i := range d.files {
		fmt.Fprintf(d.w, "\r\033[K") // clear line
		d.printFileLine(i)
		lines++

		// Subtitle: only for running files with a comment
		if d.files[i].status == "RUN" && d.files[i].currentComment != "" {
			fmt.Fprintf(d.w, "\r\033[K") // clear line
			fmt.Fprintf(d.w, "    %s%s%s\n", colorGray, truncateCmd(d.files[i].currentComment, 60), colorReset)
			lines++
		}
	}

	// Clear any leftover lines from previous render (if we shrank)
	extra := d.lastRenderedLines - lines
	for i := 0; i < extra; i++ {
		fmt.Fprintf(d.w, "\r\033[K\n")
	}
	// Move cursor back up to end of actual content
	if extra > 0 {
		fmt.Fprintf(d.w, "\033[%dA", extra)
	}

	d.lastRenderedLines = lines
	d.printed = true
}

func (d *ProgressDisplay) printFileLine(idx int) {
	f := d.files[idx]
	done := f.status == "OK" || f.status == "ERR"
	bar := renderBar(f.completed, max(f.total, 1), done)

	var statusColor string
	switch f.status {
	case "OK":
		statusColor = colorGreen
	case "ERR":
		statusColor = colorRed
	default:
		statusColor = colorYellow
	}

	// Pad status to 3 chars
	status := f.status
	if len(status) < 3 {
		status = status + strings.Repeat(" ", 3-len(status))
	}

	// Counter and timing
	var elapsed time.Duration
	if done && !f.endTime.IsZero() {
		elapsed = f.endTime.Sub(f.startTime)
	} else {
		elapsed = time.Since(f.startTime)
	}
	timeStr := formatDuration(elapsed)

	var suffix string
	if f.total > 0 {
		if done {
			suffix = fmt.Sprintf(" (%d/%d) took %s", f.completed, f.total, timeStr)
		} else {
			suffix = fmt.Sprintf(" (%d/%d) %s", f.completed, f.total, timeStr)
		}
	}

	fmt.Fprintf(d.w, "%s%s%s %s - %s%s%s%s\n", statusColor, status, colorReset, bar, f.name, colorGray, suffix, colorReset)
}

// renderBar generates a progress bar string like [=====>    ] or [==========].
func renderBar(completed, total int, done bool) string {
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


