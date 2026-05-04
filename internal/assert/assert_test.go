package assert

import (
	"testing"

	"github.com/sleipi/clit/internal/runner"
	"github.com/sleipi/clit/internal/types"
)

func TestContains(t *testing.T) {
	r := runner.Result{Stdout: "hello world\n", Stderr: "", ExitCode: 0}
	a := types.Assert{Query: "stdout", Predicate: "contains", Value: "world"}
	res := Evaluate(a, r)
	if !res.Pass {
		t.Fatalf("expected pass, got fail: %s", res.Message)
	}
}

func TestNotContains(t *testing.T) {
	r := runner.Result{Stdout: "hello world\n", Stderr: "", ExitCode: 0}
	a := types.Assert{Query: "stdout", Predicate: "contains", Value: "error", Negated: true}
	res := Evaluate(a, r)
	if !res.Pass {
		t.Fatalf("expected pass, got fail: %s", res.Message)
	}
}

func TestContainsFails(t *testing.T) {
	r := runner.Result{Stdout: "hello world\n", Stderr: "", ExitCode: 0}
	a := types.Assert{Query: "stdout", Predicate: "contains", Value: "missing"}
	res := Evaluate(a, r)
	if res.Pass {
		t.Fatal("expected fail")
	}
}

func TestMatches(t *testing.T) {
	r := runner.Result{Stdout: "2024-01-15\n", Stderr: "", ExitCode: 0}
	a := types.Assert{Query: "stdout", Predicate: "matches", Value: `\d{4}-\d{2}-\d{2}`}
	res := Evaluate(a, r)
	if !res.Pass {
		t.Fatalf("expected pass: %s", res.Message)
	}
}

func TestIsEmpty(t *testing.T) {
	r := runner.Result{Stdout: "hello\n", Stderr: "", ExitCode: 0}
	a := types.Assert{Query: "stderr", Predicate: "isEmpty"}
	res := Evaluate(a, r)
	if !res.Pass {
		t.Fatalf("expected pass: %s", res.Message)
	}
}

func TestLineQuery(t *testing.T) {
	r := runner.Result{Stdout: "first\nsecond\nthird\n", Stderr: "", ExitCode: 0}
	a := types.Assert{Query: "line 2", Predicate: "==", Value: "second"}
	res := Evaluate(a, r)
	if !res.Pass {
		t.Fatalf("expected pass: %s", res.Message)
	}
}

func TestLineCount(t *testing.T) {
	r := runner.Result{Stdout: "a\nb\nc\n", Stderr: "", ExitCode: 0}
	a := types.Assert{Query: "lineCount", Predicate: "==", Value: "3"}
	res := Evaluate(a, r)
	if !res.Pass {
		t.Fatalf("expected pass: %s", res.Message)
	}
}

func TestStartsWith(t *testing.T) {
	r := runner.Result{Stdout: "Usage: command [options]\n", Stderr: "", ExitCode: 0}
	a := types.Assert{Query: "stdout", Predicate: "startsWith", Value: "Usage:"}
	res := Evaluate(a, r)
	if !res.Pass {
		t.Fatalf("expected pass: %s", res.Message)
	}
}

func TestEqual(t *testing.T) {
	r := runner.Result{Stdout: "exact\n", Stderr: "", ExitCode: 0}
	a := types.Assert{Query: "stdout", Predicate: "==", Value: "exact"}
	res := Evaluate(a, r)
	if !res.Pass {
		t.Fatalf("expected pass: %s", res.Message)
	}
}

func TestDuration(t *testing.T) {
	r := runner.Result{Stdout: "", Stderr: "", ExitCode: 0, DurationMs: 150}
	a := types.Assert{Query: "duration", Predicate: "<", Value: "1000"}
	res := Evaluate(a, r)
	if !res.Pass {
		t.Fatalf("expected pass: %s", res.Message)
	}
}
