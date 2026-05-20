package runner

import (
	"strings"
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

func TestRunWithPrompts_SinglePrompt(t *testing.T) {
	prompts := []PromptDef{
		{Pattern: "Enter name:", IsRegex: false, Response: "Alice", Repeat: 1},
	}
	result := RunWithPrompts(`printf "Enter name: " && read name && echo "Hello $name"`, prompts, 5000)
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !contains(result.Stdout, "Hello Alice") {
		t.Fatalf("expected stdout to contain 'Hello Alice', got %q", result.Stdout)
	}
	if len(result.UnmatchedPrompts) != 0 {
		t.Fatalf("expected no unmatched prompts, got %v", result.UnmatchedPrompts)
	}
}

func TestRunWithPrompts_MultiplePrompts(t *testing.T) {
	prompts := []PromptDef{
		{Pattern: "First:", IsRegex: false, Response: "Jane", Repeat: 1},
		{Pattern: "Last:", IsRegex: false, Response: "Doe", Repeat: 1},
	}
	cmd := `printf "First: " && read f && printf "Last: " && read l && echo "Hi $f $l"`
	result := RunWithPrompts(cmd, prompts, 5000)
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !contains(result.Stdout, "Hi Jane Doe") {
		t.Fatalf("expected stdout to contain 'Hi Jane Doe', got %q", result.Stdout)
	}
}

func TestRunWithPrompts_RegexPattern(t *testing.T) {
	prompts := []PromptDef{
		{Pattern: `Continue\?`, IsRegex: true, Response: "yes", Repeat: 1},
	}
	cmd := `printf "Continue? " && read ans && echo "Got: $ans"`
	result := RunWithPrompts(cmd, prompts, 5000)
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if !contains(result.Stdout, "Got: yes") {
		t.Fatalf("expected stdout to contain 'Got: yes', got %q", result.Stdout)
	}
}

func TestRunWithPrompts_Multiplier(t *testing.T) {
	prompts := []PromptDef{
		{Pattern: "Next?", IsRegex: false, Response: "y", Repeat: 3},
	}
	cmd := `for i in 1 2 3; do printf "Next? " && read ans && echo "$ans"; done`
	result := RunWithPrompts(cmd, prompts, 5000)
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if len(result.UnmatchedPrompts) != 0 {
		t.Fatalf("expected no unmatched prompts, got %v", result.UnmatchedPrompts)
	}
}

func TestRunWithPrompts_UnmatchedPrompt(t *testing.T) {
	prompts := []PromptDef{
		{Pattern: "Never appears:", IsRegex: false, Response: "x", Repeat: 1},
	}
	result := RunWithPrompts("echo done", prompts, 5000)
	if len(result.UnmatchedPrompts) != 1 {
		t.Fatalf("expected 1 unmatched prompt, got %d", len(result.UnmatchedPrompts))
	}
	if result.UnmatchedPrompts[0] != "Never appears:" {
		t.Fatalf("expected unmatched prompt 'Never appears:', got %q", result.UnmatchedPrompts[0])
	}
}

func TestRunWithPrompts_Timeout(t *testing.T) {
	prompts := []PromptDef{
		{Pattern: "Name:", IsRegex: false, Response: "x", Repeat: 1},
	}
	// Program blocks on read but never prints "Name:", so prompt never matches
	result := RunWithPrompts("read x", prompts, 500)
	if !result.TimedOut {
		t.Fatal("expected timeout")
	}
}

func TestRunBackground_Signal(t *testing.T) {
	// Start a process that traps TERM and exits cleanly
	bp, err := RunBackground(`trap 'echo terminated; exit 0' TERM; echo ready; while true; do sleep 0.1; done`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for process to be ready
	time.Sleep(200 * time.Millisecond)
	if !strings.Contains(bp.Stdout(), "ready") {
		t.Fatalf("process not ready, stdout: %q", bp.Stdout())
	}

	err = bp.Signal(15) // SIGTERM
	if err != nil {
		t.Fatalf("unexpected error sending signal: %v", err)
	}

	// Wait for process to exit
	if !bp.Wait(2 * time.Second) {
		t.Fatal("process did not exit after TERM")
	}

	if bp.ExitCode() != 0 {
		t.Fatalf("expected exit 0, got %d", bp.ExitCode())
	}
	if !strings.Contains(bp.Stdout(), "terminated") {
		t.Fatalf("expected stdout to contain 'terminated', got %q", bp.Stdout())
	}
}

func TestRunBackground_ExitCode_Kill(t *testing.T) {
	bp, err := RunBackground("sleep 10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = bp.Signal(9) // SIGKILL

	if !bp.Wait(2 * time.Second) {
		t.Fatal("process did not exit after KILL")
	}

	if bp.ExitCode() != 137 {
		t.Fatalf("expected exit 137, got %d", bp.ExitCode())
	}
}

func TestRunBackground_Wait_Timeout(t *testing.T) {
	bp, err := RunBackground("sleep 10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer bp.Kill()

	// Should return false because process doesn't exit within 100ms
	if bp.Wait(100 * time.Millisecond) {
		t.Fatal("expected Wait to return false (timeout)")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
