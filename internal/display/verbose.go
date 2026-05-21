package display

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// VerboseDisplay formats per-entry results into a buffer.
// It does NOT write to stdout directly — callers collect the output
// and pass it to ProgressDisplay.FinishFile().
type VerboseDisplay struct {
	w       io.Writer
	verbose bool // show stdout/stderr even on pass
}

// NewVerboseDisplay creates a VerboseDisplay that writes to the given writer.
// verbose controls whether passing entries also show stdout/stderr.
func NewVerboseDisplay(w io.Writer, verbose bool) *VerboseDisplay {
	return &VerboseDisplay{w: w, verbose: verbose}
}

func (d *VerboseDisplay) BeginFile(filename string) {
	fmt.Fprintf(d.w, "%s▶ %s%s\n", ColorBold, filepath.Base(filename), ColorReset)
}

func (d *VerboseDisplay) EntryResult(info EntryInfo) {
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

func (d *VerboseDisplay) EndFile() {
	fmt.Fprintln(d.w)
}

// DeferResult prints a defer entry result.
func (d *VerboseDisplay) DeferResult(command string) {
	fmt.Fprintf(d.w, "  %s~%s %s %s[defer]%s\n",
		ColorGray, ColorReset, TruncateCmd(command, 60),
		ColorGray, ColorReset)
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
