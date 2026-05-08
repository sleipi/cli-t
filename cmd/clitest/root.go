package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
)

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
	display.PrintHeader(os.Stdout, version, dArgs, parallel, noParallel, noRecursive, verbose, varFlags.values, groupFlags, excludeGroupFlags)

	if workers > len(files) {
		workers = len(files)
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	start := time.Now()

	var results []fileResult
	if verbose {
		if isTTY {
			results = runVerboseTTY(files, workers)
		} else {
			results = runVerboseNonTTY(files, workers)
		}
	} else {
		results = runCompact(files, workers, isTTY)
	}

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

func runVerboseTTY(files []string, workers int) []fileResult {
	results := make([]fileResult, len(files))
	jobs := make(chan int, len(files))
	var wg sync.WaitGroup
	var mu sync.Mutex
	appendedLines := 0
	headerLines := len(files)

	for i := range files {
		jobs <- i
	}
	close(jobs)

	for _, f := range files {
		fmt.Printf("  %s▶ %s%s %sRUNNING%s\n", display.ColorBold, filepath.Base(f), display.ColorReset, display.ColorYellow, display.ColorReset)
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				f := files[idx]
				var buf bytes.Buffer

				parsed, err := loadAndParse(f, varFlags.values)
				if err != nil {
					fmt.Fprintf(&buf, "  %s%v%s\n", display.ColorRed, err, display.ColorReset)
					mu.Lock()
					display.OverwriteHeaderLine(os.Stdout, idx, filepath.Base(f), false, headerLines, appendedLines)
					fmt.Print(buf.String())
					appendedLines += display.CountLines(buf.String())
					mu.Unlock()
					results[idx] = fileResult{fail: 1, file: f}
					continue
				}

				if parsed.Directives.Skip {
					mu.Lock()
					display.OverwriteHeaderLine(os.Stdout, idx, filepath.Base(f), true, headerLines, appendedLines)
					mu.Unlock()
					results[idx] = fileResult{skip: len(parsed.Entries), file: f}
					continue
				}

				entries := filter.Entries(parsed, groupFlags, excludeGroupFlags)

				if len(entries) == 0 {
					mu.Lock()
					display.ClearHeaderLine(os.Stdout, idx, headerLines, appendedLines)
					mu.Unlock()
					results[idx] = fileResult{file: f, hidden: true}
					continue
				}

				vd := display.NewVerboseDisplay(&buf, true)
				vd.Start([]string{f})
				vd.BeginFile(0)
				pass, fail, skip := runEntriesVerbose(vd, entries, varFlags.values)
				vd.EndFile(0)

				mu.Lock()
				display.OverwriteHeaderLine(os.Stdout, idx, filepath.Base(f), fail == 0, headerLines, appendedLines)
				fmt.Print(buf.String())
				appendedLines += display.CountLines(buf.String())
				mu.Unlock()

				results[idx] = fileResult{pass: pass, fail: fail, skip: skip, file: f}
			}
		}()
	}
	wg.Wait()
	return results
}

func runVerboseNonTTY(files []string, workers int) []fileResult {
	type verboseResult struct {
		output string
		pass   int
		fail   int
		skip   int
		file   string
		hidden bool
	}

	results := make([]fileResult, len(files))
	vResults := make([]verboseResult, len(files))
	jobs := make(chan int, len(files))
	var wg sync.WaitGroup

	for i := range files {
		jobs <- i
	}
	close(jobs)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				f := files[idx]
				var buf bytes.Buffer
				vd := display.NewVerboseDisplay(&buf, true)
				vd.Start([]string{f})

				parsed, err := loadAndParse(f, varFlags.values)
				if err != nil {
					vd.FileError(0, err.Error())
					vResults[idx] = verboseResult{output: buf.String(), fail: 1, file: f}
					continue
				}

				if parsed.Directives.Skip {
					vResults[idx] = verboseResult{output: buf.String(), skip: len(parsed.Entries), file: f}
					continue
				}

				entries := filter.Entries(parsed, groupFlags, excludeGroupFlags)

				if len(entries) == 0 {
					vResults[idx] = verboseResult{file: f, hidden: true}
					continue
				}

				vd.BeginFile(0)
				pass, fail, skip := runEntriesVerbose(vd, entries, varFlags.values)
				vd.EndFile(0)
				vResults[idx] = verboseResult{output: buf.String(), pass: pass, fail: fail, skip: skip, file: f}
			}
		}()
	}
	wg.Wait()

	for i, r := range vResults {
		if r.hidden {
			results[i] = fileResult{file: r.file, hidden: true}
			continue
		}
		fmt.Print(r.output)
		results[i] = fileResult{pass: r.pass, fail: r.fail, skip: r.skip, file: r.file}
	}
	return results
}

func runCompact(files []string, workers int, isTTY bool) []fileResult {
	results := make([]fileResult, len(files))
	jobs := make(chan int, len(files))
	var wg sync.WaitGroup

	for i := range files {
		jobs <- i
	}
	close(jobs)

	pd := display.NewProgressDisplay(os.Stdout, isTTY)
	pd.Start(files)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				f := files[idx]

				parsed, err := loadAndParse(f, varFlags.values)
				if err != nil {
					pd.FileError(idx, err.Error())
					results[idx] = fileResult{fail: 1, file: f}
					continue
				}

				if parsed.Directives.Skip {
					pd.FinishFile(idx, true)
					results[idx] = fileResult{skip: len(parsed.Entries), file: f}
					continue
				}

				entries := filter.Entries(parsed, groupFlags, excludeGroupFlags)

				if len(entries) == 0 {
					pd.HideFile(idx)
					results[idx] = fileResult{file: f, hidden: true}
					continue
				}

				pd.UpdateProgress(idx, 0, len(entries))
				pass, fail, skip, details := runEntriesCompact(pd, idx, entries, varFlags.values)
				pd.FinishFile(idx, fail == 0)
				results[idx] = fileResult{pass: pass, fail: fail, skip: skip, file: f, failures: details}
			}
		}()
	}
	wg.Wait()
	pd.Finish()

	for _, r := range results {
		if len(r.failures) > 0 {
			display.PrintFailureDetails(os.Stdout, r.failures, r.file)
		}
	}
	return results
}
