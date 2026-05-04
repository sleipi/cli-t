package runner

import (
	"testing"
)

func TestRunSimpleCommand(t *testing.T) {
	result := Run("echo hello")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Fatalf("expected stdout %q, got %q", "hello\n", result.Stdout)
	}
	if result.Stderr != "" {
		t.Fatalf("expected empty stderr, got %q", result.Stderr)
	}
}

func TestRunExitCode(t *testing.T) {
	result := Run("exit 42")
	if result.ExitCode != 42 {
		t.Fatalf("expected exit 42, got %d", result.ExitCode)
	}
}

func TestRunStderr(t *testing.T) {
	result := Run("echo error >&2")
	if result.Stderr != "error\n" {
		t.Fatalf("expected stderr %q, got %q", "error\n", result.Stderr)
	}
}

func TestRunWithEnv(t *testing.T) {
	result := RunWithEnv("echo $MY_VAR", map[string]string{"MY_VAR": "hello"})
	if result.Stdout != "hello\n" {
		t.Fatalf("expected stdout %q, got %q", "hello\n", result.Stdout)
	}
}

func TestRunDuration(t *testing.T) {
	result := Run("sleep 0.1")
	if result.DurationMs < 50 {
		t.Fatalf("expected duration >= 50ms, got %d", result.DurationMs)
	}
}
