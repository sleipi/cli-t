package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sleipi/clit/internal/types"
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
	path := filepath.Join(dir, "test.clit")
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
	_, err := loadAndParse("/nonexistent/path.clit", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadAndParse_VarSubstitution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.clit")
	os.WriteFile(path, []byte("echo {{MSG}}\n"), 0644)

	f, err := loadAndParse(path, map[string]string{"MSG": "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Entries[0].Command != "echo hi" {
		t.Errorf("expected 'echo hi', got %q", f.Entries[0].Command)
	}
}
