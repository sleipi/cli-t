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

	// Header shows configured value, not effective
	printHeader(resolved, *parallel, *noParallel, *noRecursive, *verbose, varFlags.values)

	if workers > len(files) {
		workers = len(files)
	}

	// Timer
	start := time.Now()

	type fileResult struct {
		output string
		pass   int
		fail   int
		file   string
	}

	results := make([]fileResult, len(files))
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

				content, err := os.ReadFile(f)
				if err != nil {
					fmt.Fprintf(&buf, "Error reading %s: %v\n", f, err)
					results[idx] = fileResult{output: buf.String(), fail: 1, file: f}
					continue
				}

				input := string(content)
				input = substituteVars(input, varFlags.values)

				entries, err := parser.Parse(input)
				if err != nil {
					fmt.Fprintf(&buf, "%sError parsing %s: %v%s\n", colorRed, f, err, colorReset)
					results[idx] = fileResult{output: buf.String(), fail: 1, file: f}
					continue
				}

				fmt.Fprintf(&buf, "%s▶ %s%s\n", colorBold, filepath.Base(f), colorReset)
				filePass, fileFail := runEntries(&buf, entries, *verbose)
				fmt.Fprintln(&buf)
				results[idx] = fileResult{output: buf.String(), pass: filePass, fail: fileFail, file: f}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalPass, totalFail := 0, 0
	var failedFiles []string
	for _, r := range results {
		fmt.Print(r.output)
		totalPass += r.pass
		totalFail += r.fail
		if r.fail > 0 {
			failedFiles = append(failedFiles, r.file)
		}
	}

	// Summary
	fmt.Printf("%s━━━ Summary ━━━%s\n", colorBold, colorReset)
	fmt.Printf("  %spass: %d%s\n", colorGreen, totalPass, colorReset)
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

func printHeader(resolved []resolvedArg, parallel int, noParallel, noRecursive, verbose bool, vars map[string]string) {
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
	fmt.Println()
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func runEntries(w io.Writer, entries []types.Entry, verbose bool) (pass, fail int) {
	captures := map[string]string{}

	for _, entry := range entries {
		// Substitute captures into command
		cmd := substituteVars(entry.Command, captures)

		result := runner.Run(cmd)
		entryPass := true
		var failures []string

		// Check exit code
		if result.ExitCode != entry.ExitCode {
			entryPass = false
			failures = append(failures, fmt.Sprintf("exit code: expected %d, got %d", entry.ExitCode, result.ExitCode))
		}

		// Implicit body assert
		if len(entry.Body) > 0 {
			res := assert.EvaluateBody(entry.Body, result)
			if !res.Pass {
				entryPass = false
				failures = append(failures, res.Message)
			}
		}

		// Explicit asserts
		for _, a := range entry.Asserts {
			res := assert.Evaluate(a, result)
			if !res.Pass {
				entryPass = false
				failures = append(failures, res.Message)
			}
		}

		// Captures
		for _, c := range entry.Captures {
			val := resolveCapture(c.Query, result)
			captures[c.Name] = val
		}

		// Output
		if entryPass {
			pass++
			assertCount := len(entry.Asserts)
			if len(entry.Body) > 0 {
				assertCount++
			}
			fmt.Fprintf(w, "  %s✓%s %s %s(exit=%d, %d asserts)%s\n",
				colorGreen, colorReset, truncateCmd(cmd, 60), colorGray, result.ExitCode, assertCount, colorReset)
			if verbose {
				printOutput(w, result)
			}
		} else {
			fail++
			fmt.Fprintf(w, "  %s✗%s %s\n", colorRed, colorReset, truncateCmd(cmd, 60))
			for _, msg := range failures {
				fmt.Fprintf(w, "    %sFAIL: %s%s\n", colorRed, msg, colorReset)
			}
			printOutput(w, result)
		}
	}
	return
}

func printOutput(w io.Writer, r runner.Result) {
	if r.Stdout != "" {
		fmt.Fprintf(w, "    %s--- stdout ---%s\n", colorGray, colorReset)
		for _, line := range strings.Split(strings.TrimSuffix(r.Stdout, "\n"), "\n") {
			fmt.Fprintf(w, "    %s%s%s\n", colorGray, line, colorReset)
		}
	}
	if r.Stderr != "" {
		fmt.Fprintf(w, "    %s--- stderr ---%s\n", colorYellow, colorReset)
		for _, line := range strings.Split(strings.TrimSuffix(r.Stderr, "\n"), "\n") {
			fmt.Fprintf(w, "    %s%s%s\n", colorYellow, line, colorReset)
		}
	}
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

func resolveFiles(args []string, recursive bool) ([]string, []resolvedArg, error) {
	var files []string
	var resolved []resolvedArg
	for _, arg := range args {
		countBefore := len(files)

		// Check if arg contains glob characters
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
