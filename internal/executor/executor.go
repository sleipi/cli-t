package executor

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/sleipi/cli-t/internal/assert"
	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
	"github.com/sleipi/cli-t/internal/vars"
)

// Result holds the outcome of executing a single entry.
type Result struct {
	Pass     bool
	Failures []string
	Runner   runner.Result
}

// BackgroundResult holds information about a background process that stays alive
// for later assert evaluation and/or [Finally] section execution.
type BackgroundResult struct {
	Entry   types.Entry
	Process *runner.BackgroundProcess
	Command string // substituted command for display
}

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
	passed := true
	var failures []string

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

// BackgroundEntry runs an EXIT NEVER entry and returns both the result and
// a BackgroundResult if the process should stay alive for later/finally evaluation.
// If the process should stay alive, the Result will be Pass:true but the process
// is NOT killed — the caller is responsible for cleanup via EvaluateLaterAsserts/ExecuteFinally.
func BackgroundEntry(entry types.Entry, captures map[string]string) (Result, *BackgroundResult) {
	cmd := vars.SubstituteCaptures(entry.Command, captures)
	return backgroundEntry(entry, cmd, captures)
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

	passed := true
	var failures []string

	if pr.TimedOut {
		passed = false
		failures = append(failures, "timeout waiting for prompts")
	}

	if pr.AmbiguousMatch != "" {
		passed = false
		failures = append(failures, fmt.Sprintf("ambiguous prompt match: %s", pr.AmbiguousMatch))
	}

	for _, u := range pr.UnmatchedPrompts {
		passed = false
		failures = append(failures, fmt.Sprintf("prompt %q was never matched", u))
	}

	result := pr.Result
	// Strip trailing newline for consistency with regular runner
	result.Stdout = strings.TrimRight(result.Stdout, "\n")
	result.Stderr = strings.TrimRight(result.Stderr, "\n")

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

// hasLaterAsserts returns true if any assert in the entry has the Later modifier.
func hasLaterAsserts(entry types.Entry) bool {
	for _, a := range entry.Asserts {
		if a.Later {
			return true
		}
	}
	return false
}

// backgroundEntry starts a background process and polls non-later asserts until pass or timeout.
// If the entry has later asserts or a [Finally] section, the process is kept alive on success.
func backgroundEntry(entry types.Entry, cmd string, captures map[string]string) (Result, *BackgroundResult) {
	bp, err := runner.RunBackground(cmd)
	if err != nil {
		return Result{
			Pass:     false,
			Failures: []string{fmt.Sprintf("failed to start background process: %v", err)},
		}, nil
	}

	keepAlive := hasLaterAsserts(entry) || entry.Finally != nil

	timeout := entry.Directives.Timeout
	if timeout <= 0 {
		timeout = 30000 // default 30s
	}
	poll := entry.Directives.Poll
	if poll <= 0 {
		poll = 100 // default 100ms
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)
	ticker := time.NewTicker(time.Duration(poll) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-bp.Done():
			return backgroundExitedResult(bp), nil
		case <-ticker.C:
			result, bg, done := pollBackgroundAsserts(entry, bp, cmd, captures, keepAlive, deadline)
			if done {
				return result, bg
			}
		}
	}
}

// backgroundExitedResult returns a failure result for a process that exited unexpectedly.
func backgroundExitedResult(bp *runner.BackgroundProcess) Result {
	return Result{
		Pass:     false,
		Failures: []string{"background process exited unexpectedly"},
		Runner: runner.Result{
			Stdout: strings.TrimRight(bp.Stdout(), "\n"),
			Stderr: strings.TrimRight(bp.Stderr(), "\n"),
			Pid:    bp.Pid(),
		},
	}
}

// pollBackgroundAsserts evaluates non-later asserts on a single tick.
// Returns (result, bgResult, done). If done is false, keep polling.
func pollBackgroundAsserts(entry types.Entry, bp *runner.BackgroundProcess, cmd string, captures map[string]string, keepAlive bool, deadline time.Time) (Result, *BackgroundResult, bool) {
	lastResult := runner.Result{
		Stdout: strings.TrimRight(bp.Stdout(), "\n"),
		Stderr: strings.TrimRight(bp.Stderr(), "\n"),
		Pid:    bp.Pid(),
	}

	allPass := true
	var lastFailures []string

	if len(entry.Body) > 0 {
		res := assert.EvaluateBody(entry.Body, lastResult)
		if !res.Pass {
			allPass = false
			lastFailures = append(lastFailures, res.Message)
		}
	}

	for _, a := range entry.Asserts {
		if a.Later {
			continue
		}
		res := assert.Evaluate(a, lastResult)
		if !res.Pass {
			allPass = false
			lastFailures = append(lastFailures, res.Message)
		}
	}

	if allPass {
		for _, c := range entry.Captures {
			val := vars.ResolveCapture(c.Query, lastResult)
			captures[c.Name] = val
		}
		if keepAlive {
			return Result{Pass: true, Runner: lastResult}, &BackgroundResult{
				Entry:   entry,
				Process: bp,
				Command: cmd,
			}, true
		}
		return Result{Pass: true, Runner: lastResult}, nil, true
	}

	if time.Now().After(deadline) {
		_ = bp.Kill()
		return Result{
			Pass:     false,
			Failures: append([]string{"timeout waiting for assertions to pass"}, lastFailures...),
			Runner:   lastResult,
		}, nil, true
	}

	return Result{}, nil, false
}

// EvaluateLaterAsserts evaluates all "later" asserts for background entries
// against their accumulated output. Returns failures grouped by entry command.
func EvaluateLaterAsserts(bgs []*BackgroundResult) []LaterResult {
	var results []LaterResult
	for _, bg := range bgs {
		result := runner.Result{
			Stdout: strings.TrimRight(bg.Process.Stdout(), "\n"),
			Stderr: strings.TrimRight(bg.Process.Stderr(), "\n"),
			Pid:    bg.Process.Pid(),
		}

		var failures []string
		for _, a := range bg.Entry.Asserts {
			if !a.Later {
				continue
			}
			res := assert.Evaluate(a, result)
			if !res.Pass {
				failures = append(failures, res.Message)
			}
		}

		results = append(results, LaterResult{
			Command:  bg.Command,
			Pass:     len(failures) == 0,
			Failures: failures,
			Runner:   result,
		})
	}
	return results
}

// LaterResult holds the result of evaluating later asserts for one background entry.
type LaterResult struct {
	Command  string
	Pass     bool
	Failures []string
	Runner   runner.Result
}

// FinallyResult holds the result of executing a [Finally] section.
type FinallyResult struct {
	Command  string
	Pass     bool
	Failures []string
	Runner   runner.Result
}

// signalFromName converts a signal name to a syscall.Signal.
func signalFromName(name string) syscall.Signal {
	switch name {
	case "TERM":
		return syscall.SIGTERM
	case "KILL":
		return syscall.SIGKILL
	case "INT":
		return syscall.SIGINT
	case "HUP":
		return syscall.SIGHUP
	case "QUIT":
		return syscall.SIGQUIT
	default:
		return syscall.SIGTERM
	}
}

// ExecuteFinally executes [Finally] sections for background entries in LIFO order.
// It sends the signal, waits for the process to exit, checks exit code, and
// evaluates post-signal asserts.
func ExecuteFinally(bgs []*BackgroundResult) []FinallyResult {
	var results []FinallyResult

	// Process in reverse (LIFO) order
	for i := len(bgs) - 1; i >= 0; i-- {
		bg := bgs[i]
		if bg.Entry.Finally == nil {
			continue
		}

		fin := bg.Entry.Finally
		sig := signalFromName(fin.Signal)

		// Send signal
		if err := bg.Process.Signal(sig); err != nil {
			results = append(results, FinallyResult{
				Command:  bg.Command,
				Pass:     false,
				Failures: []string{fmt.Sprintf("failed to send %s signal: %v", fin.Signal, err)},
			})
			continue
		}

		// Wait for process to exit
		timeout := time.Duration(fin.Timeout) * time.Millisecond
		if !bg.Process.Wait(timeout) {
			results = append(results, FinallyResult{
				Command:  bg.Command,
				Pass:     false,
				Failures: []string{fmt.Sprintf("[Finally] timeout: process did not exit within %dms after %s", fin.Timeout, fin.Signal)},
			})
			continue
		}

		// Check exit code
		var failures []string
		actualExit := bg.Process.ExitCode()
		if actualExit != fin.ExitCode {
			failures = append(failures, fmt.Sprintf("[Finally] exit code: expected %d, got %d", fin.ExitCode, actualExit))
		}

		// Evaluate post-signal asserts
		result := runner.Result{
			Stdout:   strings.TrimRight(bg.Process.Stdout(), "\n"),
			Stderr:   strings.TrimRight(bg.Process.Stderr(), "\n"),
			Pid:      bg.Process.Pid(),
			ExitCode: actualExit,
		}

		for _, a := range fin.Asserts {
			res := assert.Evaluate(a, result)
			if !res.Pass {
				failures = append(failures, res.Message)
			}
		}

		results = append(results, FinallyResult{
			Command:  bg.Command,
			Pass:     len(failures) == 0,
			Failures: failures,
			Runner:   result,
		})
	}
	return results
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
