package display

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ResolvedArg holds per-argument resolution info for the header output.
type ResolvedArg struct {
	Input string
	Count int
}

// CompactFailure records a failure in compact mode for post-run reporting.
type CompactFailure struct {
	Command  string
	Failures []string
	Stdout   string
	Stderr   string
}

// PrintHeader prints the clitest run header to stdout.
func PrintHeader(w io.Writer, version string, resolved []ResolvedArg, parallel int, noParallel, noRecursive, verbose bool, vars map[string]string, groups, excludeGroups []string) {
	fmt.Fprintf(w, "clitest v%s\n", version)
	for _, r := range resolved {
		suffix := fmt.Sprintf("%d file(s) loaded", r.Count)
		if r.Count == 1 {
			suffix = "1 file loaded"
		}
		if noRecursive {
			suffix += " - no-recursive"
		}
		fmt.Fprintf(w, "  path:     %s (%s)\n", r.Input, suffix)
	}
	if noParallel {
		fmt.Fprintf(w, "  no-parallel\n")
	} else {
		fmt.Fprintf(w, "  parallel: %d\n", parallel)
	}
	if verbose {
		fmt.Fprintf(w, "  verbose:  on\n")
	}
	if len(vars) > 0 {
		names := make([]string, 0, len(vars))
		for k := range vars {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Fprintf(w, "  vars:     %s\n", strings.Join(names, ", "))
	}
	if len(groups) > 0 {
		fmt.Fprintf(w, "  group:    %s\n", strings.Join(groups, ", "))
	}
	if len(excludeGroups) > 0 {
		fmt.Fprintf(w, "  exclude:  %s\n", strings.Join(excludeGroups, ", "))
	}
	fmt.Fprintln(w)
}

// PrintSummary prints the test run summary.
func PrintSummary(w io.Writer, totalPass, totalFail, totalSkip int, failedFiles []string, elapsed time.Duration) {
	fmt.Fprintf(w, "%s━━━ Summary ━━━%s\n", ColorBold, ColorReset)
	fmt.Fprintf(w, "  %spass: %d%s\n", ColorGreen, totalPass, ColorReset)
	if totalSkip > 0 {
		fmt.Fprintf(w, "  %sskip: %d%s\n", ColorYellow, totalSkip, ColorReset)
	}
	if totalFail > 0 {
		fmt.Fprintf(w, "  %sfail: %d%s\n", ColorRed, totalFail, ColorReset)
		for _, f := range failedFiles {
			fmt.Fprintf(w, "  %s  - %s%s\n", ColorRed, filepath.Base(f), ColorReset)
		}
	}
	fmt.Fprintf(w, "  %stook: %s%s\n", ColorGray, FormatDuration(elapsed), ColorReset)
}

// PrintFailureDetails prints detailed failure output for compact mode.
func PrintFailureDetails(w io.Writer, failures []CompactFailure, file string) {
	fmt.Fprintf(w, "%s▶ %s%s\n", ColorBold, filepath.Base(file), ColorReset)
	for _, f := range failures {
		fmt.Fprintf(w, "  %s✗%s %s\n", ColorRed, ColorReset, TruncateCmd(f.Command, 60))
		for _, msg := range f.Failures {
			fmt.Fprintf(w, "    %sFAIL: %s%s\n", ColorRed, msg, ColorReset)
		}
		if f.Stdout != "" {
			fmt.Fprintf(w, "    %s--- stdout ---%s\n", ColorGray, ColorReset)
			for _, line := range strings.Split(strings.TrimSuffix(f.Stdout, "\n"), "\n") {
				fmt.Fprintf(w, "    %s%s%s\n", ColorGray, line, ColorReset)
			}
		}
		if f.Stderr != "" {
			fmt.Fprintf(w, "    %s--- stderr ---%s\n", ColorYellow, ColorReset)
			for _, line := range strings.Split(strings.TrimSuffix(f.Stderr, "\n"), "\n") {
				fmt.Fprintf(w, "    %s%s%s\n", ColorYellow, line, ColorReset)
			}
		}
	}
	fmt.Fprintln(w)
}

// OverwriteHeaderLine moves the cursor up to the header line at fileIdx,
// overwrites it with OK/FAIL status, then moves back down.
func OverwriteHeaderLine(w io.Writer, fileIdx int, filename string, passed bool, headerLines, appendedLines int) {
	cursorUp := (headerLines - fileIdx) + appendedLines
	fmt.Fprintf(w, "\033[%dA", cursorUp)
	fmt.Fprintf(w, "\r\033[K")
	if passed {
		fmt.Fprintf(w, "  %s▶ %s%s %sOK%s\n", ColorBold, filename, ColorReset, ColorGreen, ColorReset)
	} else {
		fmt.Fprintf(w, "  %s▶ %s%s %sFAIL%s\n", ColorBold, filename, ColorReset, ColorRed, ColorReset)
	}
	if cursorUp-1 > 0 {
		fmt.Fprintf(w, "\033[%dB", cursorUp-1)
	}
}

// ClearHeaderLine clears a header line (used when a file has no matching entries).
func ClearHeaderLine(w io.Writer, fileIdx, headerLines, appendedLines int) {
	cursorUp := (headerLines - fileIdx) + appendedLines
	fmt.Fprintf(w, "\033[%dA", cursorUp)
	fmt.Fprintf(w, "\r\033[K\n")
	if cursorUp-1 > 0 {
		fmt.Fprintf(w, "\033[%dB", cursorUp-1)
	}
}
