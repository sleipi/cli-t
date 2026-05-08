package display

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
	fmt.Fprintf(d.w, "%s▶ %s%s\n", ColorBold, filepath.Base(d.files[fileIdx]), ColorReset)
}

func (d *VerboseDisplay) EntryResult(fileIdx int, info EntryInfo) {
	if info.Skipped {
		reason := ""
		if info.SkipReason != "" {
			reason = fmt.Sprintf(" (%s)", info.SkipReason)
		}
		fmt.Fprintf(d.w, "  %s⊘%s %s %sSKIP%s%s\n",
			ColorYellow, ColorReset, TruncateCmd(info.Command, 60),
			ColorYellow, reason, ColorReset)
		return
	}
	if info.Passed {
		fmt.Fprintf(d.w, "  %s✓%s %s %s(exit=%d, %d asserts)%s\n",
			ColorGreen, ColorReset, TruncateCmd(info.Command, 60),
			ColorGray, info.ExitCode, info.AssertCount, ColorReset)
		if d.verbose {
			d.printOutput(info)
		}
	} else {
		fmt.Fprintf(d.w, "  %s✗%s %s\n", ColorRed, ColorReset, TruncateCmd(info.Command, 60))
		for _, msg := range info.Failures {
			fmt.Fprintf(d.w, "    %sFAIL: %s%s\n", ColorRed, msg, ColorReset)
		}
		d.printOutput(info)
	}
}

func (d *VerboseDisplay) EndFile(fileIdx int) {
	fmt.Fprintln(d.w)
}

func (d *VerboseDisplay) Finish() {}

// DeferResult prints a defer entry result in verbose mode.
func (d *VerboseDisplay) DeferResult(command string, exitCode int) {
	fmt.Fprintf(d.w, "  %s~%s %s %s[defer]%s\n",
		ColorGray, ColorReset, TruncateCmd(command, 60),
		ColorGray, ColorReset)
}

// FileError prints an error message for the file.
func (d *VerboseDisplay) FileError(fileIdx int, msg string) {
	fmt.Fprintf(d.w, "%s%s%s\n", ColorRed, msg, ColorReset)
}

func (d *VerboseDisplay) printOutput(info EntryInfo) {
	if info.Stdout != "" {
		fmt.Fprintf(d.w, "    %s--- stdout ---%s\n", ColorGray, ColorReset)
		for _, line := range strings.Split(strings.TrimSuffix(info.Stdout, "\n"), "\n") {
			fmt.Fprintf(d.w, "    %s%s%s\n", ColorGray, line, ColorReset)
		}
	}
	if info.Stderr != "" {
		fmt.Fprintf(d.w, "    %s--- stderr ---%s\n", ColorYellow, ColorReset)
		for _, line := range strings.Split(strings.TrimSuffix(info.Stderr, "\n"), "\n") {
			fmt.Fprintf(d.w, "    %s%s%s\n", ColorYellow, line, ColorReset)
		}
	}
}
