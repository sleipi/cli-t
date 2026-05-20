package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sleipi/cli-t/internal/display"
	"github.com/sleipi/cli-t/internal/executor"
	"github.com/sleipi/cli-t/internal/parser"
	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
	"github.com/sleipi/cli-t/internal/vars"
)

// runEntriesVerbose executes entries and reports to VerboseDisplay.
func runEntriesVerbose(vd *display.VerboseDisplay, entries []types.Entry, v map[string]string) (pass, fail, skip int) {
	regular, defers := executor.SplitDeferEntries(entries)
	captures := map[string]string{}
	var backgrounds []*executor.BackgroundResult

	for _, entry := range regular {
		if cancelled.Load() {
			skip++
			continue
		}

		if entry.Directives.Skip {
			skip++
			vd.EntryResult(0, display.EntryInfo{
				Command:    entry.Command,
				Skipped:    true,
				SkipReason: entry.Directives.SkipReason,
			})
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
			if failFast {
				cancelled.Store(true)
			}
		}

		vd.EntryResult(0, display.EntryInfo{
			Command:     cmd,
			Passed:      er.Pass,
			ExitCode:    er.Runner.ExitCode,
			AssertCount: assertCount,
			Failures:    er.Failures,
			Stdout:      er.Runner.Stdout,
			Stderr:      er.Runner.Stderr,
		})
	}

	fail += processBackgroundsVerbose(vd, backgrounds)

	// Execute defers and display them
	for _, entry := range defers {
		cmd := vars.SubstituteCaptures(entry.Command, captures)
		result := runner.Run(cmd)
		vd.DeferResult(cmd, result.ExitCode)
	}

	return
}

// processBackgroundsVerbose evaluates later asserts and finally sections, reporting to VerboseDisplay.
func processBackgroundsVerbose(vd *display.VerboseDisplay, backgrounds []*executor.BackgroundResult) (fail int) {
	if len(backgrounds) == 0 {
		return 0
	}

	laterResults := executor.EvaluateLaterAsserts(backgrounds)
	for _, lr := range laterResults {
		if lr.Pass {
			continue
		}
		fail++
		if failFast {
			cancelled.Store(true)
		}
		vd.EntryResult(0, display.EntryInfo{
			Command:  lr.Command,
			Passed:   false,
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
		if failFast {
			cancelled.Store(true)
		}
		vd.EntryResult(0, display.EntryInfo{
			Command:  fr.Command,
			Passed:   false,
			Failures: fr.Failures,
			Stdout:   fr.Runner.Stdout,
			Stderr:   fr.Runner.Stderr,
		})
	}

	return
}

// runEntriesCompact executes entries and reports progress to ProgressDisplay.
func runEntriesCompact(pd *display.ProgressDisplay, fileIdx int, entries []types.Entry, v map[string]string) (pass, fail, skip int, details []display.CompactFailure) {
	regular, defers := executor.SplitDeferEntries(entries)
	captures := map[string]string{}
	var backgrounds []*executor.BackgroundResult

	for i, entry := range regular {
		if cancelled.Load() {
			skip++
			pd.UpdateProgress(fileIdx, i+1, len(regular))
			continue
		}

		if entry.Directives.Skip {
			skip++
			pd.UpdateProgress(fileIdx, i+1, len(regular))
			continue
		}

		cmd := vars.SubstituteCaptures(entry.Command, captures)

		subtitle := cmd
		if entry.Comment != "" {
			subtitle = strings.TrimPrefix(entry.Comment, "# ")
		}
		pd.UpdateEntry(fileIdx, subtitle)

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

		if er.Pass {
			pass++
		} else {
			fail++
			if failFast {
				cancelled.Store(true)
			}
			details = append(details, display.CompactFailure{
				Command:  cmd,
				Failures: er.Failures,
				Stdout:   er.Runner.Stdout,
				Stderr:   er.Runner.Stderr,
			})
		}

		pd.UpdateProgress(fileIdx, i+1, len(regular))
	}

	bgFail, bgDetails := processBackgroundsCompact(backgrounds)
	fail += bgFail
	details = append(details, bgDetails...)

	// Execute defers silently in compact mode
	executor.ExecuteDefers(defers, captures)

	return
}

// processBackgroundsCompact evaluates later asserts and finally sections for compact mode.
func processBackgroundsCompact(backgrounds []*executor.BackgroundResult) (fail int, details []display.CompactFailure) {
	if len(backgrounds) == 0 {
		return 0, nil
	}

	laterResults := executor.EvaluateLaterAsserts(backgrounds)
	for _, lr := range laterResults {
		if lr.Pass {
			continue
		}
		fail++
		if failFast {
			cancelled.Store(true)
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
		if failFast {
			cancelled.Store(true)
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
