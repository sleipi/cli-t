package executor

import (
	"fmt"
	"strings"
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

// Entry runs a single entry's command and evaluates all assertions.
// It also stores any captures into the provided captures map.
func Entry(entry types.Entry, captures map[string]string) Result {
	cmd := vars.SubstituteCaptures(entry.Command, captures)

	if entry.ExitNever {
		return backgroundEntry(entry, cmd, captures)
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

// backgroundEntry starts a background process and polls asserts until pass or timeout.
func backgroundEntry(entry types.Entry, cmd string, captures map[string]string) Result {
	bp, err := runner.RunBackground(cmd)
	if err != nil {
		return Result{
			Pass:     false,
			Failures: []string{fmt.Sprintf("failed to start background process: %v", err)},
		}
	}

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

	var lastFailures []string
	var lastResult runner.Result

	for {
		select {
		case <-bp.Done():
			lastResult = runner.Result{
				Stdout: strings.TrimRight(bp.Stdout(), "\n"),
				Stderr: strings.TrimRight(bp.Stderr(), "\n"),
				Pid:    bp.Pid(),
			}
			return Result{
				Pass:     false,
				Failures: []string{"background process exited unexpectedly"},
				Runner:   lastResult,
			}
		case <-ticker.C:
			lastResult = runner.Result{
				Stdout: strings.TrimRight(bp.Stdout(), "\n"),
				Stderr: strings.TrimRight(bp.Stderr(), "\n"),
				Pid:    bp.Pid(),
			}

			allPass := true
			lastFailures = nil

			if len(entry.Body) > 0 {
				res := assert.EvaluateBody(entry.Body, lastResult)
				if !res.Pass {
					allPass = false
					lastFailures = append(lastFailures, res.Message)
				}
			}

			for _, a := range entry.Asserts {
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
				return Result{Pass: true, Runner: lastResult}
			}

			if time.Now().After(deadline) {
				_ = bp.Kill()
				return Result{
					Pass:     false,
					Failures: append([]string{"timeout waiting for assertions to pass"}, lastFailures...),
					Runner:   lastResult,
				}
			}
		}
	}
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
