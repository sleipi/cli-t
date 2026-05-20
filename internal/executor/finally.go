package executor

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/sleipi/cli-t/internal/assert"
	"github.com/sleipi/cli-t/internal/runner"
)

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
