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

// BackgroundEntry runs an EXIT NEVER entry and returns both the result and
// a BackgroundResult if the process should stay alive for later/finally evaluation.
// If the process should stay alive, the Result will be Pass:true but the process
// is NOT killed — the caller is responsible for cleanup via EvaluateLaterAsserts/ExecuteFinally.
func BackgroundEntry(entry types.Entry, captures map[string]string) (Result, *BackgroundResult) {
	cmd := vars.SubstituteCaptures(entry.Command, captures)
	return backgroundEntry(entry, cmd, captures)
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
