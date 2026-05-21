package executor

import (
	"testing"
	"time"

	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
)

func TestExecuteFinally_SignalAndExit(t *testing.T) {
	// Process that handles TERM and exits 0
	bp, err := runner.RunBackground(`sh -c 'trap "exit 0" TERM; echo ready; sleep 10 & wait'`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	// Wait for "ready" output
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bp.Stdout() != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 3000},
		},
		Process: bp,
		Command: "test-term",
	}}

	results := ExecuteFinally(bgs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Pass {
		t.Fatalf("expected pass, got: %v", results[0].Failures)
	}
}

func TestExecuteFinally_ExitCodeMismatch(t *testing.T) {
	// Process exits 1 on TERM but we expect 0
	bp, err := runner.RunBackground(`sh -c 'trap "exit 1" TERM; echo ready; sleep 10 & wait'`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bp.Stdout() != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 3000},
		},
		Process: bp,
		Command: "test-mismatch",
	}}

	results := ExecuteFinally(bgs)
	if results[0].Pass {
		t.Fatal("expected failure for exit code mismatch")
	}
	found := false
	for _, f := range results[0].Failures {
		if f == "[Finally] exit code: expected 0, got 1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected exit code mismatch message, got: %v", results[0].Failures)
	}
}

func TestExecuteFinally_Timeout(t *testing.T) {
	// Process ignores TERM
	bp, err := runner.RunBackground(`sh -c 'trap "" TERM; echo ready; sleep 10 & wait'`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bp.Stdout() != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 200},
		},
		Process: bp,
		Command: "test-timeout",
	}}

	results := ExecuteFinally(bgs)
	if results[0].Pass {
		t.Fatal("expected failure on timeout")
	}
	_ = bp.Kill()
}

func TestExecuteFinally_PostSignalAsserts(t *testing.T) {
	// Process writes "shutdown" to stderr on TERM
	bp, err := runner.RunBackground(`sh -c 'trap "echo shutdown >&2; exit 0" TERM; echo ready; sleep 10 & wait'`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bp.Stdout() != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Finally: &types.Finally{
				Signal:   "TERM",
				ExitCode: 0,
				Timeout:  3000,
				Asserts:  []types.Assert{{Query: "stderr", Predicate: "contains", Value: "shutdown"}},
			},
		},
		Process: bp,
		Command: "test-asserts",
	}}

	results := ExecuteFinally(bgs)
	if !results[0].Pass {
		t.Fatalf("expected pass, got: %v", results[0].Failures)
	}
}

func TestExecuteFinally_PostSignalAssertsFail(t *testing.T) {
	bp, err := runner.RunBackground(`sh -c 'trap "exit 0" TERM; echo ready; sleep 10 & wait'`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bp.Stdout() != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Finally: &types.Finally{
				Signal:   "TERM",
				ExitCode: 0,
				Timeout:  3000,
				Asserts:  []types.Assert{{Query: "stderr", Predicate: "contains", Value: "expected_output"}},
			},
		},
		Process: bp,
		Command: "test-assert-fail",
	}}

	results := ExecuteFinally(bgs)
	if results[0].Pass {
		t.Fatal("expected failure for unmet post-signal assert")
	}
}

func TestExecuteFinally_LIFO(t *testing.T) {
	bp1, err := runner.RunBackground(`sh -c 'trap "exit 0" TERM; echo r1; sleep 10 & wait'`)
	if err != nil {
		t.Fatalf("failed to start bp1: %v", err)
	}
	bp2, err := runner.RunBackground(`sh -c 'trap "exit 0" TERM; echo r2; sleep 10 & wait'`)
	if err != nil {
		t.Fatalf("failed to start bp2: %v", err)
	}

	// Wait for both to be ready
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bp1.Stdout() != "" && bp2.Stdout() != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{
		{
			Entry:   types.Entry{Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 3000}},
			Process: bp1,
			Command: "first",
		},
		{
			Entry:   types.Entry{Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 3000}},
			Process: bp2,
			Command: "second",
		},
	}

	results := ExecuteFinally(bgs)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// LIFO: "second" processed first
	if results[0].Command != "second" {
		t.Fatalf("expected LIFO order, first result should be 'second', got %q", results[0].Command)
	}
	if results[1].Command != "first" {
		t.Fatalf("expected LIFO order, second result should be 'first', got %q", results[1].Command)
	}
}

func TestExecuteFinally_SkipsEntriesWithoutFinally(t *testing.T) {
	bp, err := runner.RunBackground(`sh -c 'echo ready; sleep 10'`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bp.Stdout() != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{{
		Entry:   types.Entry{Finally: nil},
		Process: bp,
		Command: "no-finally",
	}}

	results := ExecuteFinally(bgs)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for entry without Finally, got %d", len(results))
	}
	_ = bp.Kill()
}
