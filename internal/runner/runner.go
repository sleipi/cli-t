package runner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Result holds the output of a command execution.
type Result struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	DurationMs int64
	Pid        int // only set for background processes
}

// BackgroundProcess represents a running background process.
type BackgroundProcess struct {
	cmd    *exec.Cmd
	stdout *syncBuffer
	stderr *syncBuffer
	done   chan struct{}
	err    error
}

// syncBuffer is a thread-safe bytes.Buffer.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

// Pid returns the process ID of the background process.
func (bp *BackgroundProcess) Pid() int {
	if bp.cmd != nil && bp.cmd.Process != nil {
		return bp.cmd.Process.Pid
	}
	return 0
}

// Stdout returns the current stdout content.
func (bp *BackgroundProcess) Stdout() string {
	return bp.stdout.String()
}

// Stderr returns the current stderr content.
func (bp *BackgroundProcess) Stderr() string {
	return bp.stderr.String()
}

// Done returns a channel that is closed when the process exits.
func (bp *BackgroundProcess) Done() <-chan struct{} {
	return bp.done
}

// Kill terminates the background process.
func (bp *BackgroundProcess) Kill() error {
	if bp.cmd != nil && bp.cmd.Process != nil {
		// Kill the entire process group
		return syscall.Kill(-bp.cmd.Process.Pid, syscall.SIGKILL)
	}
	return nil
}

// Signal sends a signal to the background process.
func (bp *BackgroundProcess) Signal(sig syscall.Signal) error {
	if bp.cmd != nil && bp.cmd.Process != nil {
		// Send signal to the process group to reach child processes
		return syscall.Kill(-bp.cmd.Process.Pid, sig)
	}
	return nil
}

// ExitCode returns the exit code of the background process.
// Must only be called after the process has exited (after <-Done() or Wait returns true).
// For signal-killed processes, returns 128 + signal number (e.g. 137 for SIGKILL).
func (bp *BackgroundProcess) ExitCode() int {
	if bp.err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(bp.err, &exitErr) {
		if code := exitErr.ExitCode(); code != -1 {
			return code
		}
		// ExitCode returns -1 for signal-killed processes; extract from WaitStatus
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
	}
	return -1
}

// Wait waits for the process to exit within the given timeout.
// Returns true if the process exited, false if the timeout was reached.
func (bp *BackgroundProcess) Wait(timeout time.Duration) bool {
	select {
	case <-bp.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// RunBackground starts a command in the background without waiting for it to exit.
func RunBackground(command string) (*BackgroundProcess, error) {
	return RunBackgroundWithEnv(command, nil)
}

// RunBackgroundWithEnv starts a command in the background with additional env vars.
func RunBackgroundWithEnv(command string, env map[string]string) (*BackgroundProcess, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	bp := &BackgroundProcess{
		cmd:    cmd,
		stdout: &syncBuffer{},
		stderr: &syncBuffer{},
		done:   make(chan struct{}),
	}

	cmd.Stdout = bp.stdout
	cmd.Stderr = bp.stderr

	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start background process: %w", err)
	}

	// Monitor process exit in background
	go func() {
		bp.err = cmd.Wait()
		close(bp.done)
	}()

	return bp, nil
}

// Run executes a command via sh -c and returns the result.
func Run(command string) Result {
	return RunWithEnv(command, nil)
}

// RunWithEnv executes a command with additional environment variables.
func RunWithEnv(command string, env map[string]string) Result {
	cmd := exec.Command("sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	return Result{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode,
		DurationMs: duration,
	}
}
