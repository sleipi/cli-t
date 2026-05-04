package runner

import (
	"bytes"
	"os"
	"os/exec"
	"time"
)

// Result holds the output of a command execution.
type Result struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	DurationMs int64
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
