package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// VerboseDisplay renders per-entry results (the original clit output style).
type VerboseDisplay struct {
	w       io.Writer
	files   []string
	verbose bool // show stdout/stderr even on pass
}

// NewVerboseDisplay creates a VerboseDisplay.
// verbose controls whether passing entries also show stdout/stderr.
func NewVerboseDisplay(w io.Writer, verbose bool) *VerboseDisplay {
	return &VerboseDisplay{w: w, verbose: verbose}
}

func (d *VerboseDisplay) Start(files []string) {
	d.files = files
}

func (d *VerboseDisplay) BeginFile(fileIdx int) {
	fmt.Fprintf(d.w, "%s▶ %s%s\n", colorBold, filepath.Base(d.files[fileIdx]), colorReset)
}

func (d *VerboseDisplay) EntryResult(fileIdx int, info EntryInfo) {
	if info.Skipped {
		reason := ""
		if info.SkipReason != "" {
			reason = fmt.Sprintf(" (%s)", info.SkipReason)
		}
		fmt.Fprintf(d.w, "  %s⊘%s %s %sSKIP%s%s\n",
			colorYellow, colorReset, truncateCmd(info.Command, 60),
			colorYellow, reason, colorReset)
		return
	}
	if info.Passed {
		fmt.Fprintf(d.w, "  %s✓%s %s %s(exit=%d, %d asserts)%s\n",
			colorGreen, colorReset, truncateCmd(info.Command, 60),
			colorGray, info.ExitCode, info.AssertCount, colorReset)
		if d.verbose {
			d.printOutput(info)
		}
	} else {
		fmt.Fprintf(d.w, "  %s✗%s %s\n", colorRed, colorReset, truncateCmd(info.Command, 60))
		for _, msg := range info.Failures {
			fmt.Fprintf(d.w, "    %sFAIL: %s%s\n", colorRed, msg, colorReset)
		}
		d.printOutput(info)
	}
}

func (d *VerboseDisplay) EndFile(fileIdx int) {
	fmt.Fprintln(d.w)
}

func (d *VerboseDisplay) Finish() {}

// FileError prints an error message for the file.
func (d *VerboseDisplay) FileError(fileIdx int, msg string) {
	fmt.Fprintf(d.w, "%s%s%s\n", colorRed, msg, colorReset)
}

func (d *VerboseDisplay) printOutput(info EntryInfo) {
	if info.Stdout != "" {
		fmt.Fprintf(d.w, "    %s--- stdout ---%s\n", colorGray, colorReset)
		for _, line := range strings.Split(strings.TrimSuffix(info.Stdout, "\n"), "\n") {
			fmt.Fprintf(d.w, "    %s%s%s\n", colorGray, line, colorReset)
		}
	}
	if info.Stderr != "" {
		fmt.Fprintf(d.w, "    %s--- stderr ---%s\n", colorYellow, colorReset)
		for _, line := range strings.Split(strings.TrimSuffix(info.Stderr, "\n"), "\n") {
			fmt.Fprintf(d.w, "    %s%s%s\n", colorYellow, line, colorReset)
		}
	}
}
