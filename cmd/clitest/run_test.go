package main

import (
	"os"
	"path/filepath"
	"testing"
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
