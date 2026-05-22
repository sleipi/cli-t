package executor

import (
	"testing"

	"github.com/sleipi/cli-t/internal/types"
)

func TestEntry_PassingCommand(t *testing.T) {
	entry := types.Entry{Command: "echo hello", ExitCode: 0}
	captures := map[string]string{}
	er := Entry(entry, captures)
	if !er.Pass {
		t.Errorf("expected pass, got failures: %v", er.Failures)
	}
}

func TestEntry_FailingExitCode(t *testing.T) {
	entry := types.Entry{Command: "exit 1", ExitCode: 0}
	captures := map[string]string{}
	er := Entry(entry, captures)
	if er.Pass {
		t.Error("expected failure")
	}
}

func TestEntry_CaptureStored(t *testing.T) {
	entry := types.Entry{
		Command:  "echo captured_value",
		ExitCode: 0,
		Captures: []types.Capture{{Name: "out", Query: "stdout"}},
	}
	captures := map[string]string{}
	er := Entry(entry, captures)
	if !er.Pass {
		t.Fatalf("expected pass, got: %v", er.Failures)
	}
	if captures["out"] != "captured_value" {
		t.Errorf("expected 'captured_value', got %q", captures["out"])
	}
}

func TestSplitDeferEntries_LIFO(t *testing.T) {
	entries := []types.Entry{
		{Command: "echo a"},
		{Command: "cleanup1", Directives: types.EntryDirectives{Defer: true}},
		{Command: "echo b"},
		{Command: "cleanup2", Directives: types.EntryDirectives{Defer: true}},
	}
	regular, defers := SplitDeferEntries(entries)
	if len(regular) != 2 {
		t.Fatalf("expected 2 regular, got %d", len(regular))
	}
	if len(defers) != 2 || defers[0].Command != "cleanup2" {
		t.Errorf("expected LIFO order, got %v", defers)
	}
}

func TestExecuteDefers_Success(t *testing.T) {
	defers := []types.Entry{
		{Command: "echo cleanup"},
	}
	logs := ExecuteDefers(defers, map[string]string{})
	if len(logs) != 0 {
		t.Errorf("expected no errors, got %v", logs)
	}
}

func TestExecuteDefers_ErrorLogged(t *testing.T) {
	defers := []types.Entry{
		{Command: "exit 1"},
	}
	logs := ExecuteDefers(defers, map[string]string{})
	if len(logs) != 1 {
		t.Fatalf("expected 1 error log, got %d", len(logs))
	}
}

func TestEntry_AssertPassing(t *testing.T) {
	entry := types.Entry{
		Command:  "echo hello world",
		ExitCode: 0,
		Asserts:  []types.Assert{{Query: "stdout", Predicate: "contains", Value: "hello"}},
	}
	er := Entry(entry, map[string]string{})
	if !er.Pass {
		t.Fatalf("expected pass, got: %v", er.Failures)
	}
}

func TestEntry_AssertFailing(t *testing.T) {
	entry := types.Entry{
		Command:  "echo hello",
		ExitCode: 0,
		Asserts:  []types.Assert{{Query: "stdout", Predicate: "contains", Value: "missing"}},
	}
	er := Entry(entry, map[string]string{})
	if er.Pass {
		t.Fatal("expected failure")
	}
}

func TestEntry_NegatedAssert(t *testing.T) {
	entry := types.Entry{
		Command:  "echo hello",
		ExitCode: 0,
		Asserts:  []types.Assert{{Query: "stdout", Predicate: "contains", Value: "error", Negated: true}},
	}
	er := Entry(entry, map[string]string{})
	if !er.Pass {
		t.Fatalf("expected pass, got: %v", er.Failures)
	}
}

func TestEntry_BodyMatch(t *testing.T) {
	entry := types.Entry{
		Command:  `printf "line1\nline2"`,
		ExitCode: 0,
		Body:     []string{"line1", "line2"},
	}
	er := Entry(entry, map[string]string{})
	if !er.Pass {
		t.Fatalf("expected pass, got: %v", er.Failures)
	}
}

func TestEntry_BodyMismatch(t *testing.T) {
	entry := types.Entry{
		Command:  "echo actual",
		ExitCode: 0,
		Body:     []string{"expected"},
	}
	er := Entry(entry, map[string]string{})
	if er.Pass {
		t.Fatal("expected failure for body mismatch")
	}
}

func TestExecuteDefers_CaptureSubstitution(t *testing.T) {
	defers := []types.Entry{
		{Command: `echo {{name}}`},
	}
	captures := map[string]string{"name": "substituted"}
	logs := ExecuteDefers(defers, captures)
	if len(logs) != 0 {
		t.Errorf("expected no errors, got %v", logs)
	}
}

func TestEntry_PromptResponds(t *testing.T) {
	entry := types.Entry{
		Command:  `printf "Enter name: " && read name && echo "Hello $name"`,
		ExitCode: 0,
		Prompts: []types.Prompt{
			{Pattern: "Enter name:", IsRegex: false, Response: "Alice", Repeat: 1},
		},
		Directives: types.EntryDirectives{Timeout: 3000},
	}
	captures := map[string]string{}
	er := Entry(entry, captures)
	if !er.Pass {
		t.Fatalf("expected pass, got failures: %v (stdout: %q)", er.Failures, er.Runner.Stdout)
	}
}
