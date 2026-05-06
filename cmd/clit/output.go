package main

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

func printHeader(resolved []resolvedArg, parallel int, noParallel, noRecursive, verbose bool, vars map[string]string, groups []string, excludeGroups []string) {
	fmt.Printf("clit v%s\n", version)
	for _, r := range resolved {
		suffix := fmt.Sprintf("%d file(s) loaded", r.count)
		if r.count == 1 {
			suffix = "1 file loaded"
		}
		if noRecursive {
			suffix += " - no-recursive"
		}
		fmt.Printf("  path:     %s (%s)\n", r.input, suffix)
	}
	if noParallel {
		fmt.Printf("  no-parallel\n")
	} else {
		fmt.Printf("  parallel: %d\n", parallel)
	}
	if verbose {
		fmt.Printf("  verbose:  on\n")
	}
	if len(vars) > 0 {
		names := make([]string, 0, len(vars))
		for k := range vars {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Printf("  vars:     %s\n", strings.Join(names, ", "))
	}
	if len(groups) > 0 {
		fmt.Printf("  group:    %s\n", strings.Join(groups, ", "))
	}
	if len(excludeGroups) > 0 {
		fmt.Printf("  exclude:  %s\n", strings.Join(excludeGroups, ", "))
	}
	fmt.Println()
}

func printSummary(totalPass, totalFail, totalSkip int, failedFiles []string, elapsed time.Duration) {
	fmt.Printf("%s━━━ Summary ━━━%s\n", colorBold, colorReset)
	fmt.Printf("  %spass: %d%s\n", colorGreen, totalPass, colorReset)
	if totalSkip > 0 {
		fmt.Printf("  %sskip: %d%s\n", colorYellow, totalSkip, colorReset)
	}
	if totalFail > 0 {
		fmt.Printf("  %sfail: %d%s\n", colorRed, totalFail, colorReset)
		for _, f := range failedFiles {
			fmt.Printf("  %s  - %s%s\n", colorRed, filepath.Base(f), colorReset)
		}
	}
	fmt.Printf("  %stook: %s%s\n", colorGray, formatDuration(elapsed), colorReset)
}

func printFailureDetails(failures []compactFailure, file string) {
	fmt.Printf("%s▶ %s%s\n", colorBold, filepath.Base(file), colorReset)
	for _, f := range failures {
		fmt.Printf("  %s✗%s %s\n", colorRed, colorReset, truncateCmd(f.command, 60))
		for _, msg := range f.failures {
			fmt.Printf("    %sFAIL: %s%s\n", colorRed, msg, colorReset)
		}
		if f.stdout != "" {
			fmt.Printf("    %s--- stdout ---%s\n", colorGray, colorReset)
			for _, line := range strings.Split(strings.TrimSuffix(f.stdout, "\n"), "\n") {
				fmt.Printf("    %s%s%s\n", colorGray, line, colorReset)
			}
		}
		if f.stderr != "" {
			fmt.Printf("    %s--- stderr ---%s\n", colorYellow, colorReset)
			for _, line := range strings.Split(strings.TrimSuffix(f.stderr, "\n"), "\n") {
				fmt.Printf("    %s%s%s\n", colorYellow, line, colorReset)
			}
		}
	}
	fmt.Println()
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func truncateCmd(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// overwriteHeaderLine moves the cursor up to the header line at fileIdx,
// overwrites it with OK/FAIL status, then moves back down.
func overwriteHeaderLine(w io.Writer, fileIdx int, filename string, passed bool, headerLines, appendedLines int) {
	cursorUp := (headerLines - fileIdx) + appendedLines
	fmt.Fprintf(w, "\033[%dA", cursorUp)
	fmt.Fprintf(w, "\r\033[K")
	if passed {
		fmt.Fprintf(w, "  %s▶ %s%s %sOK%s\n", colorBold, filename, colorReset, colorGreen, colorReset)
	} else {
		fmt.Fprintf(w, "  %s▶ %s%s %sFAIL%s\n", colorBold, filename, colorReset, colorRed, colorReset)
	}
	if cursorUp-1 > 0 {
		fmt.Fprintf(w, "\033[%dB", cursorUp-1)
	}
}

// countLines counts the number of newlines in a string.
func countLines(s string) int {
	return strings.Count(s, "\n")
}

func clearHeaderLine(w io.Writer, fileIdx int, headerLines, appendedLines int) {
	cursorUp := (headerLines - fileIdx) + appendedLines
	fmt.Fprintf(w, "\033[%dA", cursorUp)
	fmt.Fprintf(w, "\r\033[K\n")
	if cursorUp-1 > 0 {
		fmt.Fprintf(w, "\033[%dB", cursorUp-1)
	}
}
