package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sleipi/cli-t/internal/display"
	"github.com/sleipi/cli-t/internal/filter"
	"github.com/sleipi/cli-t/internal/report"
	"github.com/sleipi/cli-t/internal/resolve"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var version = "dev"

var (
	verbose           bool
	silent            bool
	noRecursive       bool
	parallel          int
	noParallel        bool
	varFlags          varMap
	groupFlags        []string
	excludeGroupFlags []string
	failFast          bool
	noColor           bool
	reportFlag        string
)

// cancelled is set to true when --fail-fast triggers; workers check before picking up new jobs.
var cancelled atomic.Bool

var rootCmd = &cobra.Command{
	Use:   "clitest [options] <file.clitest|directory>...",
	Short: "Run and test CLI commands with declarative .clitest files",
	Long:  "clitest — run and test CLI commands with declarative .clitest files",
	Example: `  clitest test.clitest
  clitest test/e2e/
  clitest --var HOST=localhost examples/
  clitest --report junit:build/clitest.xml test/`,
	Args:          cobra.MinimumNArgs(1),
	RunE:          runMain,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVarP(&silent, "silent", "s", false, "Suppress all output except summary and failures")
	rootCmd.Flags().BoolVar(&noRecursive, "no-recursive", false, "Disable recursive directory scanning")
	rootCmd.Flags().IntVar(&parallel, "parallel", 8, "Max parallel file executions")
	rootCmd.Flags().BoolVar(&noParallel, "no-parallel", false, "Disable parallel execution")
	rootCmd.Flags().Var(&varFlags, "var", "Set variable: NAME=VALUE (repeatable)")
	rootCmd.Flags().StringSliceVar(&groupFlags, "group", nil, "Run only entries with this group tag (repeatable, OR logic)")
	rootCmd.Flags().StringSliceVar(&excludeGroupFlags, "exclude-group", nil, "Skip entries with this group tag (repeatable)")
	rootCmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop after first test failure")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable ANSI color codes in output")
	rootCmd.Flags().StringVar(&reportFlag, "report", "", "Write a test report: junit:PATH")

	rootCmd.SetVersionTemplate("clitest version {{.Version}}\n")
	rootCmd.Flags().BoolP("version", "V", false, "Show version")
	rootCmd.DisableFlagsInUseLine = true
	rootCmd.MarkFlagsMutuallyExclusive("parallel", "no-parallel")
	rootCmd.MarkFlagsMutuallyExclusive("verbose", "silent")
}

type fileResult struct {
	pass     int
	fail     int
	skip     int
	display  string
	failures []display.CompactFailure
	hidden   bool
	outcomes []entryOutcome
	parseErr string
	duration time.Duration
}

func runMain(_ *cobra.Command, args []string) error {
	var reportPath string
	if reportFlag != "" {
		_, path, err := parseReport(reportFlag)
		if err != nil {
			return err
		}
		reportPath = path
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if noColor || os.Getenv("NO_COLOR") != "" || !isTTY {
		display.DisableColors()
	}

	files, resolved, warnings, err := resolve.Files(args, !noRecursive)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// Print warnings to stderr unless silent
	if !silent {
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, w)
		}
	}

	workers := parallel
	if noParallel || workers <= 0 {
		workers = 1
	}

	if !silent {
		dArgs := make([]display.ResolvedArg, len(resolved))
		for i, r := range resolved {
			dArgs[i] = display.ResolvedArg{Input: r.Input, Count: r.Count}
		}
		display.PrintHeader(os.Stdout, version, dArgs, parallel, noParallel, noRecursive, verbose, failFast, varFlags.values, groupFlags, excludeGroupFlags)
	}

	if workers > len(files) {
		workers = len(files)
	}

	start := time.Now()

	var displayWriter io.Writer
	displayTTY := isTTY
	if silent {
		displayWriter = io.Discard
		displayTTY = false
	} else {
		displayWriter = os.Stdout
	}

	results := runFiles(files, workers, displayTTY, displayWriter)

	elapsed := time.Since(start)

	totalPass, totalFail, totalSkip := 0, 0, 0
	var failedFiles []string
	for _, r := range results {
		totalPass += r.pass
		totalFail += r.fail
		totalSkip += r.skip
		if r.fail > 0 {
			failedFiles = append(failedFiles, r.display)
		}
	}

	display.PrintSummary(os.Stdout, totalPass, totalFail, totalSkip, len(warnings), failedFiles, elapsed)

	if reportPath != "" {
		if err := report.WriteJUnit(reportPath, reportSuites(results)); err != nil {
			return err
		}
	}

	if totalFail > 0 {
		os.Exit(1)
	}
	return nil
}

// runFiles is the unified execution function for both compact and verbose modes.
// It uses ProgressDisplay for the dynamic running-file block at the bottom,
// and emits finished file output (compact line or verbose block) permanently above.
func runFiles(files []resolve.ScanFile, workers int, isTTY bool, w io.Writer) []fileResult {
	results := make([]fileResult, len(files))
	jobs := make(chan int, len(files))
	var wg sync.WaitGroup

	cfg := &runConfig{
		FailFast:  failFast,
		Cancelled: &cancelled,
	}

	for i := range files {
		jobs <- i
	}
	close(jobs)

	maxDynamic := workers
	if maxDynamic > 16 {
		maxDynamic = 16
	}

	displays := make([]string, len(files))
	for i, f := range files {
		displays[i] = f.Display
	}

	pd := display.NewProgressDisplay(w, isTTY, maxDynamic)
	pd.Start(displays)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if cfg.Cancelled.Load() {
					continue
				}
				results[idx] = processFile(cfg, files[idx], idx, pd)
			}
		}()
	}
	wg.Wait()
	pd.Finish()

	// Print failure details after all files complete (compact mode only)
	if !verbose {
		for _, r := range results {
			if len(r.failures) > 0 {
				display.PrintFailureDetails(os.Stdout, r.failures, r.display)
			}
		}
	}
	return results
}

// processFile handles parsing, filtering, and execution for a single file.
func processFile(cfg *runConfig, f resolve.ScanFile, idx int, pd *display.ProgressDisplay) fileResult {
	start := time.Now()
	parsed, err := loadAndParse(f.Path, varFlags.values)
	if err != nil {
		errOutput := fmt.Sprintf("%s%v%s\n", display.ColorRed, err, display.ColorReset)
		pd.FileError(idx, errOutput)
		// In silent mode, parse errors are still printed directly
		if silent {
			fmt.Fprintf(os.Stdout, "%v\n", err)
		}
		if cfg.FailFast {
			cfg.Cancelled.Store(true)
		}
		return fileResult{fail: 1, display: f.Display, parseErr: err.Error(), duration: time.Since(start)}
	}

	if parsed.Directives.Skip {
		pd.UpdateProgress(idx, 0, 0)
		pd.FinishFile(idx, true, "")
		var outcomes []entryOutcome
		for _, e := range parsed.Entries {
			if !e.Directives.Defer {
				outcomes = append(outcomes, entryOutcome{Name: entryName(e), Command: e.Command, Skipped: true, SkipReason: parsed.Directives.SkipReason})
			}
		}
		return fileResult{skip: len(parsed.Entries), display: f.Display, outcomes: outcomes}
	}

	entries := filter.Entries(parsed, groupFlags, excludeGroupFlags)

	if len(entries) == 0 {
		pd.HideFile(idx)
		return fileResult{display: f.Display, hidden: true}
	}

	var outcomes []entryOutcome
	collect := func(o entryOutcome) { outcomes = append(outcomes, o) }

	if verbose {
		var buf bytes.Buffer
		vd := display.NewVerboseDisplay(&buf, true)
		vd.BeginFile(f.Display)
		pass, fail, skip := runEntriesVerbose(cfg, vd, pd, idx, entries, parsed.Directives.Var, collect)
		vd.EndFile()
		pd.FinishFile(idx, fail == 0, buf.String())
		return fileResult{pass: pass, fail: fail, skip: skip, display: f.Display, outcomes: outcomes, duration: time.Since(start)}
	}

	pass, fail, skip, details := runEntriesCompact(cfg, pd, idx, entries, parsed.Directives.Var, collect)
	pd.FinishFile(idx, fail == 0, "")
	return fileResult{pass: pass, fail: fail, skip: skip, display: f.Display, failures: details, outcomes: outcomes, duration: time.Since(start)}
}

// reportSuites maps file results to report suites. Files hidden by group filters
// and files never started because of --fail-fast are left out; defers are cleanup, not tests.
func reportSuites(results []fileResult) []report.Suite {
	var suites []report.Suite
	for _, r := range results {
		if r.hidden || r.display == "" {
			continue
		}
		suite := report.Suite{Name: r.display, Time: r.duration, Error: r.parseErr}
		for _, o := range r.outcomes {
			if o.Kind == outcomeDefer {
				continue
			}
			c := report.Case{
				Name:        o.Name,
				Time:        time.Duration(o.DurationMs) * time.Millisecond,
				Skipped:     o.Skipped,
				SkipMessage: o.SkipReason,
			}
			if !o.Passed && !o.Skipped {
				c.Failures = o.Failures
				if len(c.Failures) == 0 {
					c.Failures = []string{"entry failed"}
				}
				c.Stdout, c.Stderr = o.Stdout, o.Stderr
			}
			suite.Cases = append(suite.Cases, c)
		}
		suites = append(suites, suite)
	}
	return suites
}
