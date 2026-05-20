package executor

import (
	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
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
