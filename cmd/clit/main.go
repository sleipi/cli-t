package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/sleipi/clit/internal/assert"
	"github.com/sleipi/clit/internal/parser"
	"github.com/sleipi/clit/internal/runner"
	"github.com/sleipi/clit/internal/types"
)

const version = "0.1.0"

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// resolvedArg holds per-argument resolution info for the header output.
type resolvedArg struct {
	input string
	count int
}

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

	// Select display mode
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
			// Live verbose mode: fixed header block + append entries below as files finish
			var mu sync.Mutex
			appendedLines := 0
			headerLines := len(files)

			// Print header block: one RUNNING line per file
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
							skipped := len(parsed.Entries)
							results[idx] = fileResult{skip: skipped, file: f}
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
			// Non-TTY verbose: buffer and print at end (no ANSI)
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
		// Compact progress mode
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

		// Print failure details after progress bars
		for _, r := range results {
			if len(r.failures) > 0 {
				fmt.Printf("%s▶ %s%s\n", colorBold, filepath.Base(r.file), colorReset)
				for _, f := range r.failures {
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
		}
	}

	elapsed := time.Since(start)

	// Summary
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
	if totalFail > 0 {
		os.Exit(1)
	}
}

// runEntriesVerbose executes entries and reports to VerboseDisplay.
func runEntriesVerbose(vd *VerboseDisplay, entries []types.Entry, vars map[string]string) (pass, fail, skip int) {
	captures := map[string]string{}

	for _, entry := range entries {
		if entry.Skip {
			skip++
			vd.EntryResult(0, EntryInfo{
				Command: entry.Command,
				Skipped: true,
				SkipReason: entry.SkipReason,
			})
			continue
		}

		cmd := substituteCaptureVars(entry.Command, captures)
		result := runner.Run(cmd)
		entryPass := true
		var failures []string

		if result.ExitCode != entry.ExitCode {
			entryPass = false
			failures = append(failures, fmt.Sprintf("exit code: expected %d, got %d", entry.ExitCode, result.ExitCode))
		}

		if len(entry.Body) > 0 {
			res := assert.EvaluateBody(entry.Body, result)
			if !res.Pass {
				entryPass = false
				failures = append(failures, res.Message)
			}
		}

		for _, a := range entry.Asserts {
			res := assert.Evaluate(a, result)
			if !res.Pass {
				entryPass = false
				failures = append(failures, res.Message)
			}
		}

		for _, c := range entry.Captures {
			val := resolveCapture(c.Query, result)
			captures[c.Name] = val
		}

		assertCount := len(entry.Asserts)
		if len(entry.Body) > 0 {
			assertCount++
		}

		if entryPass {
			pass++
		} else {
			fail++
		}

		vd.EntryResult(0, EntryInfo{
			Command:     cmd,
			Passed:      entryPass,
			ExitCode:    result.ExitCode,
			AssertCount: assertCount,
			Failures:    failures,
			Stdout:      result.Stdout,
			Stderr:      result.Stderr,
		})
	}
	return
}

// compactFailure records a failure in compact mode for post-run reporting.
type compactFailure struct {
	command  string
	failures []string
	stdout   string
	stderr   string
}

// runEntriesCompact executes entries and reports progress to ProgressDisplay.
// Returns pass/fail/skip counts and any failure details for post-run reporting.
func runEntriesCompact(pd *ProgressDisplay, fileIdx int, entries []types.Entry, vars map[string]string) (pass, fail, skip int, details []compactFailure) {
	captures := map[string]string{}

	for i, entry := range entries {
		if entry.Skip {
			skip++
			pd.UpdateProgress(fileIdx, i+1, len(entries))
			continue
		}

		cmd := substituteCaptureVars(entry.Command, captures)

		// Set subtitle: comment (stripped of leading "# ") or command
		subtitle := cmd
		if entry.Comment != "" {
			subtitle = strings.TrimPrefix(entry.Comment, "# ")
		}
		pd.UpdateEntry(fileIdx, subtitle)

		result := runner.Run(cmd)
		entryPass := true
		var failures []string

		if result.ExitCode != entry.ExitCode {
			entryPass = false
			failures = append(failures, fmt.Sprintf("exit code: expected %d, got %d", entry.ExitCode, result.ExitCode))
		}

		if len(entry.Body) > 0 {
			res := assert.EvaluateBody(entry.Body, result)
			if !res.Pass {
				entryPass = false
				failures = append(failures, res.Message)
			}
		}

		for _, a := range entry.Asserts {
			res := assert.Evaluate(a, result)
			if !res.Pass {
				entryPass = false
				failures = append(failures, res.Message)
			}
		}

		for _, c := range entry.Captures {
			val := resolveCapture(c.Query, result)
			captures[c.Name] = val
		}

		if entryPass {
			pass++
		} else {
			fail++
			details = append(details, compactFailure{
				command:  cmd,
				failures: failures,
				stdout:   result.Stdout,
				stderr:   result.Stderr,
			})
		}

		pd.UpdateProgress(fileIdx, i+1, len(entries))
	}
	return
}

// overwriteHeaderLine moves the cursor up to the header line at fileIdx,
// overwrites it with OK/FAIL status, then moves back down.
func overwriteHeaderLine(w io.Writer, fileIdx int, filename string, passed bool, headerLines, appendedLines int) {
	// Current cursor is at the bottom (after header block + appended lines)
	// Line fileIdx is (headerLines - 1 - fileIdx) lines above end of header block
	// Plus appendedLines below header block
	cursorUp := (headerLines - fileIdx) + appendedLines

	// Move up
	fmt.Fprintf(w, "\033[%dA", cursorUp)
	// Clear and overwrite
	fmt.Fprintf(w, "\r\033[K")
	if passed {
		fmt.Fprintf(w, "  %s▶ %s%s %sOK%s\n", colorBold, filename, colorReset, colorGreen, colorReset)
	} else {
		fmt.Fprintf(w, "  %s▶ %s%s %sFAIL%s\n", colorBold, filename, colorReset, colorRed, colorReset)
	}
	// Move back down (cursorUp - 1 because the \n above already moved us one line down)
	if cursorUp-1 > 0 {
		fmt.Fprintf(w, "\033[%dB", cursorUp-1)
	}
}

func clearHeaderLine(w io.Writer, fileIdx int, headerLines, appendedLines int) {
	cursorUp := (headerLines - fileIdx) + appendedLines
	fmt.Fprintf(w, "\033[%dA", cursorUp)
	fmt.Fprintf(w, "\r\033[K\n")
	if cursorUp-1 > 0 {
		fmt.Fprintf(w, "\033[%dB", cursorUp-1)
	}
}

// countLines counts the number of newlines in a string.
func countLines(s string) int {
	return strings.Count(s, "\n")
}

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

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func resolveCapture(query string, r runner.Result) string {
	switch query {
	case "stdout":
		return strings.TrimSuffix(r.Stdout, "\n")
	case "stderr":
		return strings.TrimSuffix(r.Stderr, "\n")
	default:
		if strings.HasPrefix(query, "line ") {
			return strings.TrimSuffix(r.Stdout, "\n")
		}
		return ""
	}
}

func substituteVars(input string, vars map[string]string) string {
	result := input
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	result = os.ExpandEnv(result)
	return result
}

// substituteCaptureVars substitutes only capture variables ({{name}}) without
// expanding environment variables again (those are already expanded on the raw file content).
func substituteCaptureVars(input string, captures map[string]string) string {
	result := input
	for k, v := range captures {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
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

// filterEntries returns entries that match group/exclude-group filters.
// File-level groups are inherited by all entries.
func filterEntries(f *types.File, groups []string, excludeGroups []string) []types.Entry {
	if len(groups) == 0 && len(excludeGroups) == 0 {
		return f.Entries
	}

	var result []types.Entry
	for _, e := range f.Entries {
		effectiveTags := mergeGroups(f.Groups, e.Groups)

		if len(excludeGroups) > 0 && hasAnyTag(effectiveTags, excludeGroups) {
			continue
		}

		if len(groups) > 0 && !hasAnyTag(effectiveTags, groups) {
			continue
		}

		result = append(result, e)
	}
	return result
}

// mergeGroups returns the union of file-level and entry-level groups.
func mergeGroups(fileGroups, entryGroups []string) []string {
	if len(fileGroups) == 0 {
		return entryGroups
	}
	if len(entryGroups) == 0 {
		return fileGroups
	}
	merged := make([]string, 0, len(fileGroups)+len(entryGroups))
	merged = append(merged, fileGroups...)
	merged = append(merged, entryGroups...)
	return merged
}

// hasAnyTag checks if any of the tags is present in effectiveTags (OR logic).
func hasAnyTag(effectiveTags []string, tags []string) bool {
	for _, t := range tags {
		for _, et := range effectiveTags {
			if et == t {
				return true
			}
		}
	}
	return false
}

func resolveFiles(args []string, recursive bool) ([]string, []resolvedArg, error) {
	var files []string
	var resolved []resolvedArg
	for _, arg := range args {
		countBefore := len(files)

		if strings.ContainsAny(arg, "*?[") {
			matches, err := filepath.Glob(arg)
			if err != nil {
				return nil, nil, err
			}
			for _, m := range matches {
				info, err := os.Stat(m)
				if err != nil {
					return nil, nil, err
				}
				if info.IsDir() {
					if recursive {
						err = filepath.WalkDir(m, func(path string, d fs.DirEntry, err error) error {
							if err != nil {
								return err
							}
							if !d.IsDir() && strings.HasSuffix(path, ".clit") {
								files = append(files, path)
							}
							return nil
						})
						if err != nil {
							return nil, nil, err
						}
					} else {
						dirMatches, err := filepath.Glob(filepath.Join(m, "*.clit"))
						if err != nil {
							return nil, nil, err
						}
						files = append(files, dirMatches...)
					}
				} else {
					if !strings.HasSuffix(m, ".clit") {
						fmt.Fprintf(os.Stderr, "Warning: skipping non-.clit file: %s\n", m)
						continue
					}
					files = append(files, m)
				}
			}
			resolved = append(resolved, resolvedArg{input: arg, count: len(files) - countBefore})
			continue
		}

		info, err := os.Stat(arg)
		if err != nil {
			return nil, nil, err
		}
		if info.IsDir() {
			if recursive {
				err = filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() && strings.HasSuffix(path, ".clit") {
						files = append(files, path)
					}
					return nil
				})
				if err != nil {
					return nil, nil, err
				}
			} else {
				matches, err := filepath.Glob(filepath.Join(arg, "*.clit"))
				if err != nil {
					return nil, nil, err
				}
				files = append(files, matches...)
			}
		} else {
			if !strings.HasSuffix(arg, ".clit") {
				fmt.Fprintf(os.Stderr, "Warning: skipping non-.clit file: %s\n", arg)
				continue
			}
			files = append(files, arg)
		}
		resolved = append(resolved, resolvedArg{input: arg, count: len(files) - countBefore})
	}
	sort.Strings(files)
	return files, resolved, nil
}

func truncateCmd(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// varMap implements flag.Value for repeated --var flags
type varMap struct {
	values map[string]string
}

func (v *varMap) String() string { return "" }
func (v *varMap) Set(s string) error {
	if v.values == nil {
		v.values = make(map[string]string)
	}
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid var format, use NAME=VALUE")
	}
	v.values[parts[0]] = parts[1]
	return nil
}

// stringSlice implements flag.Value for repeated string flags (--group, --exclude-group)
type stringSlice struct {
	values []string
}

func (s *stringSlice) String() string { return strings.Join(s.values, ", ") }
func (s *stringSlice) Set(v string) error {
	s.values = append(s.values, v)
	return nil
}
