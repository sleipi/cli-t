package resolve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFiles_SingleFile(t *testing.T) {
	// Create a temp .clitest file
	dir := t.TempDir()
	f := filepath.Join(dir, "test.clitest")
	os.WriteFile(f, []byte("$ echo hi\n"), 0o644)

	files, resolved, err := Files([]string{f}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(resolved) != 1 || resolved[0].Count != 1 {
		t.Errorf("expected 1 resolved arg with count 1, got %v", resolved)
	}
}

func TestFiles_NonClitestSkipped(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "readme.md")
	os.WriteFile(f, []byte("hello"), 0o644)

	files, resolved, err := Files([]string{f}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
	if len(resolved) != 0 {
		t.Fatalf("expected 0 resolved args, got %d", len(resolved))
	}
}

func TestFiles_Directory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.clitest"), []byte("$ echo a\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.clitest"), []byte("$ echo b\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "c.clitest"), []byte("$ echo c\n"), 0o644)

	files, _, err := Files([]string{dir}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files (recursive), got %d", len(files))
	}

	files, _, err = Files([]string{dir}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files (non-recursive), got %d", len(files))
	}
}
