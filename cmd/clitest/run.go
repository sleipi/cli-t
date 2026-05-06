package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sleipi/cli-t/internal/assert"
	"github.com/sleipi/cli-t/internal/parser"
	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
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

	if entry.ExitNever {
		return executeBackgroundEntry(entry, cmd, captures)
	}

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

// executeBackgroundEntry starts a background process and polls asserts until pass or timeout.
func executeBackgroundEntry(entry types.Entry, cmd string, captures map[string]string) entryResult {
	bp, err := runner.RunBackground(cmd)
	if err != nil {
		return entryResult{
			pass:     false,
			failures: []string{fmt.Sprintf("failed to start background process: %v", err)},
		}
	}

	timeout := entry.Timeout
	if timeout <= 0 {
		timeout = 30000 // default 30s
	}
	poll := entry.Poll
	if poll <= 0 {
		poll = 100 // default 100ms
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)
	ticker := time.NewTicker(time.Duration(poll) * time.Millisecond)
	defer ticker.Stop()

	var lastFailures []string
	var lastResult runner.Result

	for {
		select {
		case <-bp.Done():
			// Process exited unexpectedly
			lastResult = runner.Result{
				Stdout: strings.TrimRight(bp.Stdout(), "\n"),
				Stderr: strings.TrimRight(bp.Stderr(), "\n"),
				Pid:    bp.Pid(),
			}
			return entryResult{
				pass:     false,
				failures: []string{"background process exited unexpectedly"},
				result:   lastResult,
			}
		case <-ticker.C:
			lastResult = runner.Result{
				Stdout: strings.TrimRight(bp.Stdout(), "\n"),
				Stderr: strings.TrimRight(bp.Stderr(), "\n"),
				Pid:    bp.Pid(),
			}

			allPass := true
			lastFailures = nil

			if len(entry.Body) > 0 {
				res := assert.EvaluateBody(entry.Body, lastResult)
				if !res.Pass {
					allPass = false
					lastFailures = append(lastFailures, res.Message)
				}
			}

			for _, a := range entry.Asserts {
				res := assert.Evaluate(a, lastResult)
				if !res.Pass {
					allPass = false
					lastFailures = append(lastFailures, res.Message)
				}
			}

			if allPass {
				// Store captures
				for _, c := range entry.Captures {
					val := resolveCapture(c.Query, lastResult)
					captures[c.Name] = val
				}
				return entryResult{pass: true, result: lastResult}
			}

			if time.Now().After(deadline) {
				_ = bp.Kill()
				return entryResult{
					pass:     false,
					failures: append([]string{"timeout waiting for assertions to pass"}, lastFailures...),
					result:   lastResult,
				}
			}
		}
	}
}

// splitDeferEntries separates defer entries from regular entries.
// Returns (regular, defers) where defers are in LIFO order.
func splitDeferEntries(entries []types.Entry) (regular, defers []types.Entry) {
	for _, e := range entries {
		if e.Defer {
			defers = append(defers, e)
		} else {
			regular = append(regular, e)
		}
	}
	// Reverse defers for LIFO execution
	for i, j := 0, len(defers)-1; i < j; i, j = i+1, j-1 {
		defers[i], defers[j] = defers[j], defers[i]
	}
	return
}

// executeDeferEntries runs all defer entries, logging errors but not failing.
func executeDeferEntries(defers []types.Entry, captures map[string]string) []string {
	var logs []string
	for _, entry := range defers {
		cmd := substituteCaptureVars(entry.Command, captures)
		result := runner.Run(cmd)
		if result.ExitCode != 0 {
			logs = append(logs, fmt.Sprintf("defer %q: exit code %d", cmd, result.ExitCode))
		}
	}
	return logs
}

// runEntriesVerbose executes entries and reports to VerboseDisplay.
func runEntriesVerbose(vd *VerboseDisplay, entries []types.Entry, vars map[string]string) (pass, fail, skip int) {
	regular, defers := splitDeferEntries(entries)
	captures := map[string]string{}

	for _, entry := range regular {
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

	// Execute defers and display them
	for _, entry := range defers {
		cmd := substituteCaptureVars(entry.Command, captures)
		result := runner.Run(cmd)
		vd.DeferResult(cmd, result.ExitCode)
	}

	return
}

// runEntriesCompact executes entries and reports progress to ProgressDisplay.
func runEntriesCompact(pd *ProgressDisplay, fileIdx int, entries []types.Entry, vars map[string]string) (pass, fail, skip int, details []compactFailure) {
	regular, defers := splitDeferEntries(entries)
	captures := map[string]string{}

	for i, entry := range regular {
		if entry.Skip {
			skip++
			pd.UpdateProgress(fileIdx, i+1, len(regular))
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

		pd.UpdateProgress(fileIdx, i+1, len(regular))
	}

	// Execute defers silently in compact mode
	executeDeferEntries(defers, captures)

	return
}

// loadAndParse reads a .clitest file, substitutes variables, and parses it into a File.
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
