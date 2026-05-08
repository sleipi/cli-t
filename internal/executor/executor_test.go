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
