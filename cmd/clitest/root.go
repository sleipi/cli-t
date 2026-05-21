package main

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sleipi/cli-t/internal/display"
	"github.com/sleipi/cli-t/internal/filter"
	"github.com/sleipi/cli-t/internal/resolve"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var version = "dev"

var (
	verbose           bool
	noRecursive       bool
	parallel          int
	noParallel        bool
	varFlags          varMap
	groupFlags        []string
	excludeGroupFlags []string
	failFast          bool
	noColor           bool
)

// cancelled is set to true when --fail-fast triggers; workers check before picking up new jobs.
var cancelled atomic.Bool

var rootCmd = &cobra.Command{
	Use:   "clitest [options] <file.clitest|directory>...",
	Short: "Run and test CLI commands with declarative .clitest files",
	Long:  "clitest — run and test CLI commands with declarative .clitest files",
	Example: `  clitest test.clitest
  clitest test/e2e/
  clitest --var HOST=localhost examples/`,
	Args:          cobra.MinimumNArgs(1),
	RunE:          runMain,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVar(&noRecursive, "no-recursive", false, "Disable recursive directory scanning")
	rootCmd.Flags().IntVar(&parallel, "parallel", 8, "Max parallel file executions")
	rootCmd.Flags().BoolVar(&noParallel, "no-parallel", false, "Disable parallel execution")
	rootCmd.Flags().Var(&varFlags, "var", "Set variable: NAME=VALUE (repeatable)")
	rootCmd.Flags().StringSliceVar(&groupFlags, "group", nil, "Run only entries with this group tag (repeatable, OR logic)")
	rootCmd.Flags().StringSliceVar(&excludeGroupFlags, "exclude-group", nil, "Skip entries with this group tag (repeatable)")
	rootCmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop after first test failure")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable ANSI color codes in output")

	rootCmd.SetVersionTemplate("clitest version {{.Version}}\n")
	rootCmd.Flags().BoolP("version", "V", false, "Show version")
	rootCmd.DisableFlagsInUseLine = true
	rootCmd.MarkFlagsMutuallyExclusive("parallel", "no-parallel")
}

type fileResult struct {
	pass     int
	fail     int
	skip     int
	file     string
	failures []display.CompactFailure
	hidden   bool
}

func runMain(_ *cobra.Command, args []string) error {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if noColor || os.Getenv("NO_COLOR") != "" || !isTTY {
		display.DisableColors()
	}

	files, resolved, err := resolve.Files(args, !noRecursive)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	workers := parallel
	if noParallel || workers <= 0 {
		workers = 1
	}

	dArgs := make([]display.ResolvedArg, len(resolved))
	for i, r := range resolved {
		dArgs[i] = display.ResolvedArg{Input: r.Input, Count: r.Count}
	}
	display.PrintHeader(os.Stdout, version, dArgs, parallel, noParallel, noRecursive, verbose, failFast, varFlags.values, groupFlags, excludeGroupFlags)

	if workers > len(files) {
		workers = len(files)
	}

	start := time.Now()

	results := runFiles(files, workers, isTTY)

	elapsed := time.Since(start)

	totalPass, totalFail, totalSkip := 0, 0, 0
	var failedFiles []string
	for _, r := range results {
		totalPass += r.pass
		totalFail += r.fail
		totalSkip += r.skip
		if r.fail > 0 {
			failedFiles = append(failedFiles, r.file)
		}
	}

	display.PrintSummary(os.Stdout, totalPass, totalFail, totalSkip, failedFiles, elapsed)

	if totalFail > 0 {
		os.Exit(1)
	}
	return nil
}

// runFiles is the unified execution function for both compact and verbose modes.
// It uses ProgressDisplay for the dynamic running-file block at the bottom,
// and emits finished file output (compact line or verbose block) permanently above.
func runFiles(files []string, workers int, isTTY bool) []fileResult {
	results := make([]fileResult, len(files))
	jobs := make(chan int, len(files))
	var wg sync.WaitGroup

	for i := range files {
		jobs <- i
	}
	close(jobs)

	maxDynamic := workers
	if maxDynamic > 16 {
		maxDynamic = 16
	}

	pd := display.NewProgressDisplay(os.Stdout, isTTY, maxDynamic)
	pd.Start(files)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if cancelled.Load() {
					continue
				}
				f := files[idx]

				parsed, err := loadAndParse(f, varFlags.values)
				if err != nil {
					errOutput := fmt.Sprintf("%s%v%s\n", display.ColorRed, err, display.ColorReset)
					pd.FileError(idx, errOutput)
					results[idx] = fileResult{fail: 1, file: f}
					if failFast {
						cancelled.Store(true)
					}
					continue
				}

			if parsed.Directives.Skip {
				pd.UpdateProgress(idx, 0, 0)
				pd.FinishFile(idx, true, "")
				results[idx] = fileResult{skip: len(parsed.Entries), file: f}
				continue
			}

				entries := filter.Entries(parsed, groupFlags, excludeGroupFlags)

				if len(entries) == 0 {
					pd.HideFile(idx)
					results[idx] = fileResult{file: f, hidden: true}
					continue
				}

				if verbose {
					var buf bytes.Buffer
					vd := display.NewVerboseDisplay(&buf, true)
					vd.BeginFile(f)
					pass, fail, skip := runEntriesVerbose(vd, pd, idx, entries)
					vd.EndFile()
					pd.FinishFile(idx, fail == 0, buf.String())
					results[idx] = fileResult{pass: pass, fail: fail, skip: skip, file: f}
			} else {
				pass, fail, skip, details := runEntriesCompact(pd, idx, entries)
				pd.FinishFile(idx, fail == 0, "")
				results[idx] = fileResult{pass: pass, fail: fail, skip: skip, file: f, failures: details}
			}
			}
		}()
	}
	wg.Wait()
	pd.Finish()

	// Print failure details after all files complete (compact mode only)
	if !verbose {
		for _, r := range results {
			if len(r.failures) > 0 {
				display.PrintFailureDetails(os.Stdout, r.failures, r.file)
			}
		}
	}
	return results
}


