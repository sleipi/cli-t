package runner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"
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
		return bp.cmd.Process.Kill()
	}
	return nil
}

// RunBackground starts a command in the background without waiting for it to exit.
func RunBackground(command string) (*BackgroundProcess, error) {
	return RunBackgroundWithEnv(command, nil)
}

// RunBackgroundWithEnv starts a command in the background with additional env vars.
func RunBackgroundWithEnv(command string, env map[string]string) (*BackgroundProcess, error) {
	cmd := exec.Command("sh", "-c", command)

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
		if exitErr, ok := err.(*exec.ExitError); ok {
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
