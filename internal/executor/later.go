package executor

import (
	"strings"

	"github.com/sleipi/cli-t/internal/assert"
	"github.com/sleipi/cli-t/internal/runner"
)

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
