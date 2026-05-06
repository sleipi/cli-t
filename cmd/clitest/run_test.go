package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sleipi/cli-t/internal/types"
)

func TestExecuteEntry_PassingCommand(t *testing.T) {
	entry := types.Entry{Command: "echo hello", ExitCode: 0}
	captures := map[string]string{}
	er := executeEntry(entry, captures)
	if !er.pass {
		t.Errorf("expected pass, got failures: %v", er.failures)
	}
	if er.result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", er.result.ExitCode)
	}
}

func TestExecuteEntry_FailingExitCode(t *testing.T) {
	entry := types.Entry{Command: "exit 1", ExitCode: 0}
	captures := map[string]string{}
	er := executeEntry(entry, captures)
	if er.pass {
		t.Error("expected failure")
	}
	if len(er.failures) == 0 {
		t.Error("expected failure messages")
	}
}

func TestExecuteEntry_CaptureStored(t *testing.T) {
	entry := types.Entry{
		Command:  "echo captured_value",
		ExitCode: 0,
		Captures: []types.Capture{{Name: "out", Query: "stdout"}},
	}
	captures := map[string]string{}
	er := executeEntry(entry, captures)
	if !er.pass {
		t.Fatalf("expected pass, got: %v", er.failures)
	}
	if captures["out"] != "captured_value" {
		t.Errorf("expected 'captured_value', got %q", captures["out"])
	}
}

func TestExecuteEntry_CaptureSubstitution(t *testing.T) {
	captures := map[string]string{"prev": "world"}
	entry := types.Entry{Command: "echo {{prev}}", ExitCode: 0, Captures: []types.Capture{{Name: "result", Query: "stdout"}}}
	er := executeEntry(entry, captures)
	if !er.pass {
		t.Fatalf("expected pass, got: %v", er.failures)
	}
	if captures["result"] != "world" {
		t.Errorf("expected 'world', got %q", captures["result"])
	}
}

func TestLoadAndParse_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.clitest")
	os.WriteFile(path, []byte("echo hello\n"), 0644)

	f, err := loadAndParse(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f.Entries))
	}
	if f.Entries[0].Command != "echo hello" {
		t.Errorf("expected 'echo hello', got %q", f.Entries[0].Command)
	}
}

func TestLoadAndParse_FileNotFound(t *testing.T) {
	_, err := loadAndParse("/nonexistent/path.clitest", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadAndParse_VarSubstitution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.clitest")
	os.WriteFile(path, []byte("echo {{MSG}}\n"), 0644)

	f, err := loadAndParse(path, map[string]string{"MSG": "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Entries[0].Command != "echo hi" {
		t.Errorf("expected 'echo hi', got %q", f.Entries[0].Command)
	}
}

func TestSplitDeferEntries(t *testing.T) {
	entries := []types.Entry{
		{Command: "echo a"},
		{Command: "cleanup1", Directives: types.EntryDirectives{Defer: true}},
		{Command: "echo b"},
		{Command: "cleanup2", Directives: types.EntryDirectives{Defer: true}},
	}
	regular, defers := splitDeferEntries(entries)

	if len(regular) != 2 {
		t.Fatalf("expected 2 regular, got %d", len(regular))
	}
	if len(defers) != 2 {
		t.Fatalf("expected 2 defers, got %d", len(defers))
	}
	// LIFO: cleanup2 first
	if defers[0].Command != "cleanup2" {
		t.Errorf("expected LIFO order, got %q first", defers[0].Command)
	}
	if defers[1].Command != "cleanup1" {
		t.Errorf("expected cleanup1 second, got %q", defers[1].Command)
	}
}

func TestSplitDeferEntries_NoDefers(t *testing.T) {
	entries := []types.Entry{
		{Command: "echo a"},
		{Command: "echo b"},
	}
	regular, defers := splitDeferEntries(entries)
	if len(regular) != 2 {
		t.Fatalf("expected 2 regular, got %d", len(regular))
	}
	if len(defers) != 0 {
		t.Fatalf("expected 0 defers, got %d", len(defers))
	}
}

func TestExecuteEntry_BackgroundProcess(t *testing.T) {
	entry := types.Entry{
		Command:   `sh -c 'echo "ready"; sleep 10'`,
		ExitNever: true,
		Directives: types.EntryDirectives{
			Timeout: 2000,
			Poll:    50,
		},
		Asserts: []types.Assert{
			{Query: "stdout", Predicate: "contains", Value: "ready"},
		},
		Captures: []types.Capture{{Name: "bgpid", Query: "pid"}},
	}
	captures := map[string]string{}
	er := executeEntry(entry, captures)
	if !er.pass {
		t.Fatalf("expected pass, got failures: %v", er.failures)
	}
	if captures["bgpid"] == "" || captures["bgpid"] == "0" {
		t.Errorf("expected valid pid capture, got %q", captures["bgpid"])
	}
}

func TestExecuteEntry_BackgroundTimeout(t *testing.T) {
	entry := types.Entry{
		Command:   "sleep 999",
		ExitNever: true,
		Directives: types.EntryDirectives{
			Timeout: 200,
			Poll:    50,
		},
		Asserts: []types.Assert{
			{Query: "stdout", Predicate: "contains", Value: "never_appears"},
		},
	}
	captures := map[string]string{}
	er := executeEntry(entry, captures)
	if er.pass {
		t.Fatal("expected failure due to timeout")
	}
}

func TestExecuteDeferEntries(t *testing.T) {
	defers := []types.Entry{
		{Command: "echo cleanup1", Directives: types.EntryDirectives{Defer: true}},
		{Command: "echo cleanup2", Directives: types.EntryDirectives{Defer: true}},
	}
	captures := map[string]string{}
	logs := executeDeferEntries(defers, captures)
	if len(logs) != 0 {
		t.Errorf("expected no error logs, got %v", logs)
	}
}

func TestExecuteDeferEntries_ErrorLogged(t *testing.T) {
	defers := []types.Entry{
		{Command: "exit 1", Directives: types.EntryDirectives{Defer: true}},
	}
	captures := map[string]string{}
	logs := executeDeferEntries(defers, captures)
	if len(logs) != 1 {
		t.Fatalf("expected 1 error log, got %d", len(logs))
	}
}
