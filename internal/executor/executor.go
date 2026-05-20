package executor

import (
	"fmt"
	"strings"

	"github.com/sleipi/cli-t/internal/assert"
	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
	"github.com/sleipi/cli-t/internal/vars"
)

// Entry runs a single entry's command and evaluates all assertions.
// It also stores any captures into the provided captures map.
func Entry(entry types.Entry, captures map[string]string) Result {
	cmd := vars.SubstituteCaptures(entry.Command, captures)

	if entry.ExitNever {
		res, _ := backgroundEntry(entry, cmd, captures)
		return res
	}

	if len(entry.Prompts) > 0 {
		return promptEntry(entry, cmd, captures)
	}

	result := runner.Run(cmd)
	return evaluateResult(entry, result, captures, nil)
}

// promptEntry runs a command with interactive prompts.
func promptEntry(entry types.Entry, cmd string, captures map[string]string) Result {
	timeout := entry.Directives.Timeout
	if timeout <= 0 {
		timeout = 30000 // default 30s
	}

	prompts := make([]runner.PromptDef, len(entry.Prompts))
	for i, p := range entry.Prompts {
		prompts[i] = runner.PromptDef{
			Pattern:  p.Pattern,
			IsRegex:  p.IsRegex,
			Response: p.Response,
			Repeat:   p.Repeat,
		}
	}

	pr := runner.RunWithPrompts(cmd, prompts, timeout)

	var failures []string

	if pr.TimedOut {
		failures = append(failures, "timeout waiting for prompts")
	}

	if pr.AmbiguousMatch != "" {
		failures = append(failures, fmt.Sprintf("ambiguous prompt match: %s", pr.AmbiguousMatch))
	}

	for _, u := range pr.UnmatchedPrompts {
		failures = append(failures, fmt.Sprintf("prompt %q was never matched", u))
	}

	result := pr.Result
	// Strip trailing newline for consistency with regular runner
	result.Stdout = strings.TrimRight(result.Stdout, "\n")
	result.Stderr = strings.TrimRight(result.Stderr, "\n")

	return evaluateResult(entry, result, captures, failures)
}

// evaluateResult checks exit code, body, asserts, and captures against a runner result.
// Any pre-existing failures (e.g. from prompt timeouts) are passed via prefailures.
func evaluateResult(entry types.Entry, result runner.Result, captures map[string]string, prefailures []string) Result {
	passed := len(prefailures) == 0
	failures := prefailures

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
		val := vars.ResolveCapture(c.Query, result)
		captures[c.Name] = val
	}

	return Result{Pass: passed, Failures: failures, Runner: result}
}

// SplitDeferEntries separates defer entries from regular entries.
// Returns (regular, defers) where defers are in LIFO order.
func SplitDeferEntries(entries []types.Entry) (regular, defers []types.Entry) {
	for _, e := range entries {
		if e.Directives.Defer {
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

// ExecuteDefers runs all defer entries, logging errors but not failing.
func ExecuteDefers(defers []types.Entry, captures map[string]string) []string {
	var logs []string
	for _, entry := range defers {
		cmd := vars.SubstituteCaptures(entry.Command, captures)
		result := runner.Run(cmd)
		if result.ExitCode != 0 {
			logs = append(logs, fmt.Sprintf("defer %q: exit code %d", cmd, result.ExitCode))
		}
	}
	return logs
}
