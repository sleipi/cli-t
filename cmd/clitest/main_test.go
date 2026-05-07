package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFiles_SingleFile(t *testing.T) {
	files, resolved, err := resolveFiles([]string{"../../examples/01_basic.clitest"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved arg, got %d", len(resolved))
	}
	if resolved[0].input != "../../examples/01_basic.clitest" {
		t.Errorf("expected input '../../examples/01_basic.clitest', got %q", resolved[0].input)
	}
	if resolved[0].count != 1 {
		t.Errorf("expected count 1, got %d", resolved[0].count)
	}
}

func TestResolveFiles_Directory(t *testing.T) {
	files, resolved, err := resolveFiles([]string{"../../test/_fixtures/resolve"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if resolved[0].input != "../../test/_fixtures/resolve" {
		t.Errorf("expected input '../../test/_fixtures/resolve', got %q", resolved[0].input)
	}
	if resolved[0].count != 2 {
		t.Errorf("expected count 2, got %d", resolved[0].count)
	}
}

func TestResolveFiles_DirectoryNoRecursive(t *testing.T) {
	files, resolved, err := resolveFiles([]string{"../../test/_fixtures/resolve"}, false)
	if err != nil {
		t.Fatal(err)
	}
	// no-recursive: only top-level .clitest in nested/
	if len(files) != 1 {
		t.Fatalf("expected 1 file (non-recursive), got %d: %v", len(files), files)
	}
	if resolved[0].count != 1 {
		t.Errorf("expected count 1, got %d", resolved[0].count)
	}
}

func TestResolveFiles_GlobPattern(t *testing.T) {
	files, resolved, err := resolveFiles([]string{"../../examples/0*.clitest"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 3 {
		t.Fatalf("expected at least 3 files matching 0*.clitest, got %d", len(files))
	}
	if resolved[0].input != "../../examples/0*.clitest" {
		t.Errorf("expected input '../../examples/0*.clitest', got %q", resolved[0].input)
	}
	if resolved[0].count != len(files) {
		t.Errorf("expected count %d, got %d", len(files), resolved[0].count)
	}
}

func TestResolveFiles_MultipleArgs(t *testing.T) {
	files, resolved, err := resolveFiles([]string{
		"../../examples/01_basic.clitest",
		"../../examples/02_errors.clitest",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved args, got %d", len(resolved))
	}
	if resolved[0].count != 1 || resolved[1].count != 1 {
		t.Errorf("expected count 1 each, got %d and %d", resolved[0].count, resolved[1].count)
	}
}

func TestResolveFiles_NonClitFileSkipped(t *testing.T) {
	// Create a temp non-.clitest file
	tmp := filepath.Join(t.TempDir(), "readme.md")
	os.WriteFile(tmp, []byte("hello"), 0644)

	files, resolved, err := resolveFiles([]string{tmp}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files (non-.clitest skipped), got %d", len(files))
	}
	if len(resolved) != 0 {
		t.Fatalf("expected 0 resolved args for skipped file, got %d", len(resolved))
	}
}

func TestResolveFiles_GlobWithDirectories(t *testing.T) {
	// ../../test/_fixtures/resolve is a directory matched by glob
	files, resolved, err := resolveFiles([]string{"../../test/_fixtures/resolv*"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files from nested/, got %d: %v", len(files), files)
	}
	if resolved[0].input != "../../test/_fixtures/resolv*" {
		t.Errorf("expected input '../../test/_fixtures/resolv*', got %q", resolved[0].input)
	}
}
