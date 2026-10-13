package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Outcome kinds that are not a plain entry result.
const (
	outcomeLater     = "later"
	outcomeFinally   = "finally"
	outcomeDefer     = "defer"
	outcomeCancelled = "cancelled"
)

// entryOutcome holds the result of executing a single entry for reporting.
type entryOutcome struct {
	Name        string
	Kind        string
	Command     string
	Passed      bool
	Skipped     bool
	SkipReason  string
	ExitCode    int
	AssertCount int
	Failures    []string
	Warnings    []string
	Stdout      string
	Stderr      string
	DurationMs  int64
}

// entryName is the first comment line of an entry, or its first command line.
func entryName(entry types.Entry) string {
	if entry.Comment != "" {
		first, _, _ := strings.Cut(entry.Comment, "\n")
		if name := strings.TrimSpace(strings.TrimLeft(first, "#")); name != "" {
			return name
		}
	}
	first, _, _ := strings.Cut(entry.Command, "\n")
	return strings.TrimSpace(first)
}

// cancelledOutcome reports an entry that did not run because --fail-fast stopped the run.
// An entry with a skip directive keeps its own reason.
func cancelledOutcome(entry types.Entry) entryOutcome {
	reason := "cancelled by --fail-fast"
	if entry.Directives.Skip {
		reason = entry.Directives.SkipReason
	}
	return entryOutcome{
		Name:       entryName(entry),
		Kind:       outcomeCancelled,
		Command:    entry.Command,
		Skipped:    true,
		SkipReason: reason,
	}
}

// runEntries executes entries, reports progress, and returns counters + failure details.
// The onResult callback is invoked for each entry outcome (including skipped entries).
// It may be nil when nobody needs the outcomes (compact mode without --report).
func runEntries(cfg *runConfig, pd *display.ProgressDisplay, fileIdx int, entries []types.Entry, fileVars map[string]string, onResult func(entryOutcome)) (pass, fail, skip int, details []display.CompactFailure) {
	regular, defers := executor.SplitDeferEntries(entries)
	pd.UpdateProgress(fileIdx, 0, len(regular))
	captures := vars.SeedCaptures(fileVars)
	var backgrounds []*executor.BackgroundResult

	for i, entry := range regular {
		if cfg.Cancelled.Load() {
			skip++
			if onResult != nil {
				onResult(cancelledOutcome(entry))
			}
			pd.UpdateProgress(fileIdx, i+1, len(regular))
			continue
		}

		if entry.Directives.Skip {
			skip++
			if onResult != nil {
				onResult(entryOutcome{
					Name:       entryName(entry),
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
				Name:        entryName(entry),
				Command:     cmd,
				Passed:      er.Pass,
				ExitCode:    er.Runner.ExitCode,
				AssertCount: assertCount,
				Failures:    er.Failures,
				Warnings:    er.Warnings,
				Stdout:      er.Runner.Stdout,
				Stderr:      er.Runner.Stderr,
				DurationMs:  er.Runner.DurationMs,
			})
		}

		pd.UpdateProgress(fileIdx, i+1, len(regular))
	}

	// Process backgrounds (later asserts + finally)
	bgFail, bgDetails := processBackgrounds(cfg, backgrounds, onResult)
	fail += bgFail
	details = append(details, bgDetails...)

	// Execute defers
	for _, entry := range defers {
		cmd := vars.SubstituteCaptures(entry.Command, captures)
		runner.Run(cmd, entry.Directives.Workdir)
		if onResult != nil {
			onResult(entryOutcome{Name: entryName(entry), Kind: outcomeDefer, Command: cmd, Passed: true, Skipped: true, SkipReason: "defer"})
		}
	}

	return
}

// runEntriesVerbose executes entries, reports to VerboseDisplay and passes every outcome to collect.
func runEntriesVerbose(cfg *runConfig, vd *display.VerboseDisplay, pd *display.ProgressDisplay, fileIdx int, entries []types.Entry, fileVars map[string]string, collect func(entryOutcome)) (pass, fail, skip int) {
	onResult := func(o entryOutcome) {
		if collect != nil {
			collect(o)
		}
		if o.Kind == outcomeCancelled {
			return
		}
		if o.Kind == outcomeDefer {
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
			Warnings:    o.Warnings,
			Stdout:      o.Stdout,
			Stderr:      o.Stderr,
		})
	}
	pass, fail, skip, _ = runEntries(cfg, pd, fileIdx, entries, fileVars, onResult)
	return
}

// processBackgrounds evaluates later asserts and finally sections for all kept-alive background processes.
// Each failure is also passed to onResult, named after its entry and marked with its kind.
func processBackgrounds(cfg *runConfig, backgrounds []*executor.BackgroundResult, onResult func(entryOutcome)) (fail int, details []display.CompactFailure) {
	if len(backgrounds) == 0 {
		return 0, nil
	}

	recordFailure := func(kind string, entry types.Entry, command string, failures []string, r runner.Result) {
		fail++
		if cfg.FailFast {
			cfg.Cancelled.Store(true)
		}
		details = append(details, display.CompactFailure{
			Command:  command,
			Failures: failures,
			Stdout:   r.Stdout,
			Stderr:   r.Stderr,
		})
		if onResult != nil {
			onResult(entryOutcome{
				Name:     entryName(entry) + " (" + kind + ")",
				Kind:     kind,
				Command:  command,
				Failures: failures,
				Stdout:   r.Stdout,
				Stderr:   r.Stderr,
			})
		}
	}

	for _, lr := range executor.EvaluateLaterAsserts(backgrounds) {
		if !lr.Pass {
			recordFailure(outcomeLater, lr.Entry, lr.Command, lr.Failures, lr.Runner)
		}
	}

	for _, fr := range executor.ExecuteFinally(backgrounds) {
		if !fr.Pass {
			recordFailure(outcomeFinally, fr.Entry, fr.Command, fr.Failures, fr.Runner)
		}
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
	f, errs := parser.ParseFile(input)
	if len(errs) > 0 {
		// Check if these are DirectiveErrors (validation) or structural parse errors
		var de *parser.DirectiveError
		if errors.As(errs[0], &de) {
			// Validation errors: format with file prefix
			msgs := make([]string, len(errs))
			for i, e := range errs {
				msgs[i] = fmt.Sprintf("%s:%s", path, e.Error())
			}
			return nil, fmt.Errorf("%s", strings.Join(msgs, "\n"))
		}
		// Structural parse errors: use legacy format
		return nil, fmt.Errorf("parsing %s: %w", path, errs[0])
	}
	f.Path = path
	if err := resolveWorkdirs(f); err != nil {
		return nil, fmt.Errorf("resolving workdir in %s: %w", path, err)
	}
	resolveEnvs(f)
	resolveTimeouts(f)
	return f, nil
}

// resolveWorkdirs resolves @workdir directives to absolute paths relative to the file's directory.
// Entry-level @workdir overrides file-level; both resolve relative to the .clitest file location.
func resolveWorkdirs(f *types.File) error {
	fileDir := filepath.Dir(f.Path)

	// Resolve file-level workdir to absolute
	fileWorkdir := f.Directives.Workdir
	if fileWorkdir != "" && !filepath.IsAbs(fileWorkdir) {
		fileWorkdir = filepath.Join(fileDir, fileWorkdir)
	}

	for i := range f.Entries {
		entryWorkdir := f.Entries[i].Directives.Workdir
		if entryWorkdir != "" {
			// Entry-level: resolve relative to file dir
			if !filepath.IsAbs(entryWorkdir) {
				entryWorkdir = filepath.Join(fileDir, entryWorkdir)
			}
		} else {
			// Inherit file-level
			entryWorkdir = fileWorkdir
		}

		if entryWorkdir != "" {
			cleanPath := filepath.Clean(entryWorkdir)
			info, err := os.Stat(cleanPath)
			if err != nil {
				return fmt.Errorf("@workdir %q: directory does not exist", f.Entries[i].Directives.Workdir)
			}
			if !info.IsDir() {
				return fmt.Errorf("@workdir %q: not a directory", cleanPath)
			}
			entryWorkdir = cleanPath
		}

		f.Entries[i].Directives.Workdir = entryWorkdir
	}
	return nil
}

// resolveEnvs merges file-level @env into each entry's Env map.
// Entry-level values override file-level for the same key.
func resolveEnvs(f *types.File) {
	if len(f.Directives.Env) == 0 {
		return
	}
	for i := range f.Entries {
		if f.Entries[i].Directives.Env == nil {
			// No entry-level env: copy file-level directly
			merged := make(map[string]string, len(f.Directives.Env))
			for k, v := range f.Directives.Env {
				merged[k] = v
			}
			f.Entries[i].Directives.Env = merged
		} else {
			// Entry-level exists: file-level fills in missing keys
			for k, v := range f.Directives.Env {
				if _, exists := f.Entries[i].Directives.Env[k]; !exists {
					f.Entries[i].Directives.Env[k] = v
				}
			}
		}
	}
}

// resolveTimeouts inherits file-level @timeout into entries that don't define their own.
func resolveTimeouts(f *types.File) {
	if f.Directives.Timeout == nil {
		return
	}
	for i := range f.Entries {
		if f.Entries[i].Directives.Timeout == nil {
			f.Entries[i].Directives.Timeout = f.Directives.Timeout
		}
	}
}
