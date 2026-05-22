package main

import (
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/sleipi/cli-t/internal/display"
	"github.com/sleipi/cli-t/internal/types"
)

func TestLoadAndParse_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.clitest")
	os.WriteFile(path, []byte("echo hello\n"), 0o644)

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
	os.WriteFile(path, []byte("echo {{MSG}}\n"), 0o644)

	f, err := loadAndParse(path, map[string]string{"MSG": "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Entries[0].Command != "echo hi" {
		t.Errorf("expected 'echo hi', got %q", f.Entries[0].Command)
	}
}

// newTestProgressDisplay creates a no-op ProgressDisplay suitable for unit tests.
func newTestProgressDisplay() *display.ProgressDisplay {
	pd := display.NewProgressDisplay(io.Discard, false, 1)
	pd.Start([]string{"test.clitest"})
	return pd
}

func TestRunEntries_FailFastCancels(t *testing.T) {
	entries := []types.Entry{
		{Command: "false", Asserts: []types.Assert{{Query: "exit", Predicate: "eq", Value: "0"}}},
		{Command: "echo second", Asserts: []types.Assert{{Query: "exit", Predicate: "eq", Value: "0"}}},
		{Command: "echo third", Asserts: []types.Assert{{Query: "exit", Predicate: "eq", Value: "0"}}},
	}

	cancelled := &atomic.Bool{}
	cfg := &runConfig{FailFast: true, Cancelled: cancelled}
	pd := newTestProgressDisplay()

	pass, fail, skip, _ := runEntries(cfg, pd, 0, entries, nil)

	if fail != 1 {
		t.Errorf("expected 1 failure, got %d", fail)
	}
	if skip != 2 {
		t.Errorf("expected 2 skipped, got %d", skip)
	}
	if pass != 0 {
		t.Errorf("expected 0 pass, got %d", pass)
	}
	if !cancelled.Load() {
		t.Error("expected Cancelled to be true after fail-fast")
	}
}

func TestRunEntries_PreCancelledSkipsAll(t *testing.T) {
	entries := []types.Entry{
		{Command: "echo first", Asserts: []types.Assert{{Query: "exit", Predicate: "eq", Value: "0"}}},
		{Command: "echo second", Asserts: []types.Assert{{Query: "exit", Predicate: "eq", Value: "0"}}},
	}

	cancelled := &atomic.Bool{}
	cancelled.Store(true)
	cfg := &runConfig{FailFast: false, Cancelled: cancelled}
	pd := newTestProgressDisplay()

	pass, fail, skip, _ := runEntries(cfg, pd, 0, entries, nil)

	if skip != 2 {
		t.Errorf("expected 2 skipped, got %d", skip)
	}
	if pass != 0 {
		t.Errorf("expected 0 pass, got %d", pass)
	}
	if fail != 0 {
		t.Errorf("expected 0 fail, got %d", fail)
	}
}

func TestProcessBackgrounds_FailFastOnLaterFailure(t *testing.T) {
	// We cannot easily construct a real BackgroundResult with a running process,
	// so we test with an empty slice to verify no panic, and rely on E2E tests
	// for the full background+later interaction.
	cancelled := &atomic.Bool{}
	cfg := &runConfig{FailFast: true, Cancelled: cancelled}

	fail, details := processBackgrounds(cfg, nil)
	if fail != 0 {
		t.Errorf("expected 0 fail for empty backgrounds, got %d", fail)
	}
	if len(details) != 0 {
		t.Errorf("expected no details, got %d", len(details))
	}
}
