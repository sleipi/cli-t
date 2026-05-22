package main

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/sleipi/cli-t/internal/display"
	"github.com/sleipi/cli-t/internal/executor"
	"github.com/sleipi/cli-t/internal/parser"
	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
	"github.com/sleipi/cli-t/internal/vars"
)

// runConfig holds runtime configuration that controls execution behaviour.
// Passing this explicitly (rather than reading package globals) makes the
// execution functions testable in isolation.
type runConfig struct {
	FailFast  bool
	Cancelled *atomic.Bool
}

// entryOutcome holds the result of executing a single entry for reporting.
type entryOutcome struct {
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

// runEntries executes entries, reports progress, and returns counters + failure details.
// The onResult callback is invoked for each entry outcome (including skipped entries).
// In verbose mode, pass a callback that writes to VerboseDisplay; in compact mode, pass nil.
func runEntries(cfg *runConfig, pd *display.ProgressDisplay, fileIdx int, entries []types.Entry, onResult func(entryOutcome)) (pass, fail, skip int, details []display.CompactFailure) {
	regular, defers := executor.SplitDeferEntries(entries)
	pd.UpdateProgress(fileIdx, 0, len(regular))
	captures := map[string]string{}
	var backgrounds []*executor.BackgroundResult

	for i, entry := range regular {
		if cfg.Cancelled.Load() {
			skip++
			pd.UpdateProgress(fileIdx, i+1, len(regular))
			continue
		}

		if entry.Directives.Skip {
			skip++
			if onResult != nil {
				onResult(entryOutcome{
					Command:    entry.Command,
					Skipped:    true,
					SkipReason: entry.Directives.SkipReason,
				})
			}
			pd.UpdateProgress(fileIdx, i+1, len(regular))
			continue
		}

		cmd := vars.SubstituteCaptures(entry.Command, captures)

		var er executor.Result
		var bg *executor.BackgroundResult

		if entry.ExitNever {
			er, bg = executor.BackgroundEntry(entry, captures)
			if bg != nil {
				backgrounds = append(backgrounds, bg)
			}
		} else {
			er = executor.Entry(entry, captures)
		}

		assertCount := len(entry.Asserts)
		if len(entry.Body) > 0 {
			assertCount++
		}

		if er.Pass {
			pass++
		} else {
			fail++
			if cfg.FailFast {
				cfg.Cancelled.Store(true)
			}
			details = append(details, display.CompactFailure{
				Command:  cmd,
				Failures: er.Failures,
				Stdout:   er.Runner.Stdout,
				Stderr:   er.Runner.Stderr,
			})
		}

		if onResult != nil {
			onResult(entryOutcome{
				Command:     cmd,
				Passed:      er.Pass,
				ExitCode:    er.Runner.ExitCode,
				AssertCount: assertCount,
				Failures:    er.Failures,
				Stdout:      er.Runner.Stdout,
				Stderr:      er.Runner.Stderr,
			})
		}

		pd.UpdateProgress(fileIdx, i+1, len(regular))
	}

	// Process backgrounds (later asserts + finally)
	bgFail, bgDetails := processBackgrounds(cfg, backgrounds)
	fail += bgFail
	details = append(details, bgDetails...)
	if onResult != nil {
		for _, d := range bgDetails {
			onResult(entryOutcome{
				Command:  d.Command,
				Passed:   false,
				Failures: d.Failures,
				Stdout:   d.Stdout,
				Stderr:   d.Stderr,
			})
		}
	}

	// Execute defers
	for _, entry := range defers {
		cmd := vars.SubstituteCaptures(entry.Command, captures)
		runner.Run(cmd)
		if onResult != nil {
			onResult(entryOutcome{Command: cmd, Passed: true, Skipped: true, SkipReason: "defer"})
		}
	}

	return
}

// runEntriesVerbose executes entries and reports to VerboseDisplay.
func runEntriesVerbose(cfg *runConfig, vd *display.VerboseDisplay, pd *display.ProgressDisplay, fileIdx int, entries []types.Entry) (pass, fail, skip int) {
	onResult := func(o entryOutcome) {
		if o.SkipReason == "defer" {
			vd.DeferResult(o.Command)
			return
		}
		vd.EntryResult(display.EntryInfo{
			Command:     o.Command,
			Passed:      o.Passed,
			Skipped:     o.Skipped,
			SkipReason:  o.SkipReason,
			ExitCode:    o.ExitCode,
			AssertCount: o.AssertCount,
			Failures:    o.Failures,
			Stdout:      o.Stdout,
			Stderr:      o.Stderr,
		})
	}
	pass, fail, skip, _ = runEntries(cfg, pd, fileIdx, entries, onResult)
	return
}

// runEntriesCompact executes entries and reports progress to ProgressDisplay.
func runEntriesCompact(cfg *runConfig, pd *display.ProgressDisplay, fileIdx int, entries []types.Entry) (pass, fail, skip int, details []display.CompactFailure) {
	return runEntries(cfg, pd, fileIdx, entries, nil)
}

// processBackgrounds evaluates later asserts and finally sections for all kept-alive background processes.
func processBackgrounds(cfg *runConfig, backgrounds []*executor.BackgroundResult) (fail int, details []display.CompactFailure) {
	if len(backgrounds) == 0 {
		return 0, nil
	}

	laterResults := executor.EvaluateLaterAsserts(backgrounds)
	for _, lr := range laterResults {
		if lr.Pass {
			continue
		}
		fail++
		if cfg.FailFast {
			cfg.Cancelled.Store(true)
		}
		details = append(details, display.CompactFailure{
			Command:  lr.Command,
			Failures: lr.Failures,
			Stdout:   lr.Runner.Stdout,
			Stderr:   lr.Runner.Stderr,
		})
	}

	finallyResults := executor.ExecuteFinally(backgrounds)
	for _, fr := range finallyResults {
		if fr.Pass {
			continue
		}
		fail++
		if cfg.FailFast {
			cfg.Cancelled.Store(true)
		}
		details = append(details, display.CompactFailure{
			Command:  fr.Command,
			Failures: fr.Failures,
			Stdout:   fr.Runner.Stdout,
			Stderr:   fr.Runner.Stderr,
		})
	}

	return
}

// loadAndParse reads a .clitest file, substitutes variables, and parses it into a File.
func loadAndParse(path string, v map[string]string) (*types.File, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	input := vars.Substitute(string(content), v)
	f, err := parser.ParseFile(input)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	f.Path = path
	return f, nil
}
