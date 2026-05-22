package executor

import (
	"strings"
	"testing"
	"time"

	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
)

func TestEvaluateLaterAsserts_Pass(t *testing.T) {
	// Start a process that outputs both "ready" and "later_output"
	bp, err := runner.RunBackground(`sh -c 'echo ready; echo later_output; sleep 10'`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer bp.Kill()
	// Wait for output to accumulate
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(bp.Stdout(), "later_output") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Asserts: []types.Assert{
				{Query: "stdout", Predicate: "contains", Value: "ready"},
				{Query: "stdout", Predicate: "contains", Value: "later_output", Later: true},
			},
		},
		Process: bp,
		Command: "test",
	}}

	results := EvaluateLaterAsserts(bgs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Pass {
		t.Fatalf("expected pass, got failures: %v", results[0].Failures)
	}
}

func TestEvaluateLaterAsserts_Fail(t *testing.T) {
	bp, err := runner.RunBackground(`sh -c 'echo ready; sleep 10'`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer bp.Kill()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(bp.Stdout(), "ready") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Asserts: []types.Assert{
				{Query: "stdout", Predicate: "contains", Value: "never_appears", Later: true},
			},
		},
		Process: bp,
		Command: "test",
	}}

	results := EvaluateLaterAsserts(bgs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Pass {
		t.Fatal("expected failure")
	}
	if len(results[0].Failures) == 0 {
		t.Fatal("expected at least one failure message")
	}
}

func TestEvaluateLaterAsserts_SkipsNonLater(t *testing.T) {
	bp, err := runner.RunBackground(`sh -c 'echo ready; sleep 10'`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer bp.Kill()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(bp.Stdout(), "ready") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Asserts: []types.Assert{
				{Query: "stdout", Predicate: "contains", Value: "ready"},           // non-later, should be skipped
				{Query: "stdout", Predicate: "contains", Value: "ready", Later: true}, // later, should be evaluated
			},
		},
		Process: bp,
		Command: "test",
	}}

	results := EvaluateLaterAsserts(bgs)
	if !results[0].Pass {
		t.Fatalf("expected pass, got: %v", results[0].Failures)
	}
}
