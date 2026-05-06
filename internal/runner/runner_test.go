package runner

import (
	"testing"
	"time"
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

func TestRunBackground_StartsProcess(t *testing.T) {
	bp, err := RunBackground("echo ready; sleep 10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer bp.Kill()

	if bp.Pid() == 0 {
		t.Fatal("expected non-zero pid")
	}

	// Wait a bit for output
	time.Sleep(200 * time.Millisecond)
	if bp.Stdout() == "" {
		t.Fatal("expected stdout to contain output")
	}
}

func TestRunBackground_Pid(t *testing.T) {
	bp, err := RunBackground("sleep 10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer bp.Kill()

	if bp.Pid() <= 0 {
		t.Fatalf("expected positive pid, got %d", bp.Pid())
	}
}

func TestRunBackground_Kill(t *testing.T) {
	bp, err := RunBackground("sleep 10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = bp.Kill()
	if err != nil {
		t.Fatalf("unexpected error killing process: %v", err)
	}

	// Process should exit after kill
	select {
	case <-bp.Done():
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit after kill")
	}
}

func TestRunBackground_Done(t *testing.T) {
	bp, err := RunBackground("echo done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-bp.Done():
		// good — short command exits immediately
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit")
	}
}

func TestRunBackground_Stderr(t *testing.T) {
	bp, err := RunBackground("echo err >&2; sleep 10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer bp.Kill()

	time.Sleep(200 * time.Millisecond)
	if bp.Stderr() == "" {
		t.Fatal("expected stderr output")
	}
}
