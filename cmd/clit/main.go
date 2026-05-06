package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/term"
)

const version = "0.1.0"

func main() {
	verbose := flag.Bool("v", false, "verbose output")
	noRecursive := flag.Bool("no-recursive", false, "disable recursive directory scanning")
	parallel := flag.Int("parallel", 8, "max parallel file executions")
	noParallel := flag.Bool("no-parallel", false, "disable parallel execution")
	varFlags := &varMap{}
	flag.Var(varFlags, "var", "set variable: --var NAME=VALUE")
	groupFlags := &stringSlice{}
	flag.Var(groupFlags, "group", "run only entries with this group tag (repeatable, OR logic)")
	excludeGroupFlags := &stringSlice{}
	flag.Var(excludeGroupFlags, "exclude-group", "skip entries with this group tag (repeatable)")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: clit [options] <file.clit|directory> ...\n")
		os.Exit(2)
	}

	files, resolved, err := resolveFiles(args, !*noRecursive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	workers := *parallel
	if *noParallel || workers <= 0 {
		workers = 1
	}

	printHeader(resolved, *parallel, *noParallel, *noRecursive, *verbose, varFlags.values, groupFlags.values, excludeGroupFlags.values)

	if workers > len(files) {
		workers = len(files)
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	useVerbose := *verbose
	start := time.Now()

	type fileResult struct {
		pass     int
		fail     int
		skip     int
		file     string
		failures []compactFailure
		hidden   bool
	}

	results := make([]fileResult, len(files))
	jobs := make(chan int, len(files))
	var wg sync.WaitGroup

	for i := range files {
		jobs <- i
	}
	close(jobs)

	if useVerbose {
		if isTTY {
			var mu sync.Mutex
			appendedLines := 0
			headerLines := len(files)

			for _, f := range files {
				fmt.Printf("  %s▶ %s%s %sRUNNING%s\n", colorBold, filepath.Base(f), colorReset, colorYellow, colorReset)
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
							buf.WriteString(fmt.Sprintf("  %s%v%s\n", colorRed, err, colorReset))
							mu.Lock()
							overwriteHeaderLine(os.Stdout, idx, filepath.Base(f), false, headerLines, appendedLines)
							fmt.Print(buf.String())
							appendedLines += countLines(buf.String())
							mu.Unlock()
							results[idx] = fileResult{fail: 1, file: f}
							continue
						}

						if parsed.Skip {
							mu.Lock()
							overwriteHeaderLine(os.Stdout, idx, filepath.Base(f), true, headerLines, appendedLines)
							mu.Unlock()
							results[idx] = fileResult{skip: len(parsed.Entries), file: f}
							continue
						}

					entries := filterEntries(parsed, groupFlags.values, excludeGroupFlags.values)

					if len(entries) == 0 {
						mu.Lock()
						clearHeaderLine(os.Stdout, idx, headerLines, appendedLines)
						mu.Unlock()
						results[idx] = fileResult{file: f, hidden: true}
						continue
					}

					vd := NewVerboseDisplay(&buf, true)
					vd.Start([]string{f})
					vd.BeginFile(0)
					pass, fail, skip := runEntriesVerbose(vd, entries, varFlags.values)
					vd.EndFile(0)

					mu.Lock()
					overwriteHeaderLine(os.Stdout, idx, filepath.Base(f), fail == 0, headerLines, appendedLines)
					fmt.Print(buf.String())
					appendedLines += countLines(buf.String())
					mu.Unlock()

					results[idx] = fileResult{pass: pass, fail: fail, skip: skip, file: f}
					}
				}()
			}
			wg.Wait()
		} else {
			type verboseResult struct {
				output string
				pass   int
				fail   int
				skip   int
				file   string
				hidden bool
			}
			vResults := make([]verboseResult, len(files))

			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for idx := range jobs {
						f := files[idx]
						var buf bytes.Buffer
						vd := NewVerboseDisplay(&buf, true)
						vd.Start([]string{f})

						parsed, err := loadAndParse(f, varFlags.values)
						if err != nil {
							vd.FileError(0, err.Error())
							vResults[idx] = verboseResult{output: buf.String(), fail: 1, file: f}
							continue
						}

						if parsed.Skip {
							vResults[idx] = verboseResult{output: buf.String(), skip: len(parsed.Entries), file: f}
							continue
						}

					entries := filterEntries(parsed, groupFlags.values, excludeGroupFlags.values)

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
		}
	} else {
		dynamic := isTTY
		pd := NewProgressDisplay(os.Stdout, dynamic)
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

					if parsed.Skip {
						pd.FinishFile(idx, true)
						results[idx] = fileResult{skip: len(parsed.Entries), file: f}
						continue
					}

				entries := filterEntries(parsed, groupFlags.values, excludeGroupFlags.values)

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
				printFailureDetails(r.failures, r.file)
			}
		}
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

	printSummary(totalPass, totalFail, totalSkip, failedFiles, elapsed)

	if totalFail > 0 {
		os.Exit(1)
	}
}
