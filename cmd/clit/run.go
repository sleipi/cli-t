package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sleipi/clit/internal/assert"
	"github.com/sleipi/clit/internal/parser"
	"github.com/sleipi/clit/internal/runner"
	"github.com/sleipi/clit/internal/types"
)

// compactFailure records a failure in compact mode for post-run reporting.
type compactFailure struct {
	command  string
	failures []string
	stdout   string
	stderr   string
}

// entryResult holds the outcome of executing a single entry.
type entryResult struct {
	pass     bool
	failures []string
	result   runner.Result
}

// executeEntry runs a single entry's command and evaluates all assertions.
// It also stores any captures into the provided captures map.
func executeEntry(entry types.Entry, captures map[string]string) entryResult {
	cmd := substituteCaptureVars(entry.Command, captures)
	result := runner.Run(cmd)
	passed := true
	var failures []string

	if result.ExitCode != entry.ExitCode {
		passed = false
		failures = append(failures, fmt.Sprintf("exit code: expected %d, got %d", entry.ExitCode, result.ExitCode))
	}

	if len(entry.Body) > 0 {
		res := assert.EvaluateBody(entry.Body, result)
		if !res.Pass {
			passed = false
			failures = append(failures, res.Message)
		}
	}

	for _, a := range entry.Asserts {
		res := assert.Evaluate(a, result)
		if !res.Pass {
			passed = false
			failures = append(failures, res.Message)
		}
	}

	for _, c := range entry.Captures {
		val := resolveCapture(c.Query, result)
		captures[c.Name] = val
	}

	return entryResult{pass: passed, failures: failures, result: result}
}

// runEntriesVerbose executes entries and reports to VerboseDisplay.
func runEntriesVerbose(vd *VerboseDisplay, entries []types.Entry, vars map[string]string) (pass, fail, skip int) {
	captures := map[string]string{}

	for _, entry := range entries {
		if entry.Skip {
			skip++
			vd.EntryResult(0, EntryInfo{
				Command:    entry.Command,
				Skipped:    true,
				SkipReason: entry.SkipReason,
			})
			continue
		}

		cmd := substituteCaptureVars(entry.Command, captures)
		er := executeEntry(entry, captures)

		assertCount := len(entry.Asserts)
		if len(entry.Body) > 0 {
			assertCount++
		}

		if er.pass {
			pass++
		} else {
			fail++
		}

		vd.EntryResult(0, EntryInfo{
			Command:     cmd,
			Passed:      er.pass,
			ExitCode:    er.result.ExitCode,
			AssertCount: assertCount,
			Failures:    er.failures,
			Stdout:      er.result.Stdout,
			Stderr:      er.result.Stderr,
		})
	}
	return
}

// runEntriesCompact executes entries and reports progress to ProgressDisplay.
func runEntriesCompact(pd *ProgressDisplay, fileIdx int, entries []types.Entry, vars map[string]string) (pass, fail, skip int, details []compactFailure) {
	captures := map[string]string{}

	for i, entry := range entries {
		if entry.Skip {
			skip++
			pd.UpdateProgress(fileIdx, i+1, len(entries))
			continue
		}

		cmd := substituteCaptureVars(entry.Command, captures)

		subtitle := cmd
		if entry.Comment != "" {
			subtitle = strings.TrimPrefix(entry.Comment, "# ")
		}
		pd.UpdateEntry(fileIdx, subtitle)

		er := executeEntry(entry, captures)

		if er.pass {
			pass++
		} else {
			fail++
			details = append(details, compactFailure{
				command:  cmd,
				failures: er.failures,
				stdout:   er.result.Stdout,
				stderr:   er.result.Stderr,
			})
		}

		pd.UpdateProgress(fileIdx, i+1, len(entries))
	}
	return
}

// loadAndParse reads a .clit file, substitutes variables, and parses it into a File.
func loadAndParse(path string, vars map[string]string) (*types.File, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	input := substituteVars(string(content), vars)
	f, err := parser.ParseFile(input)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	f.Path = path
	return f, nil
}
