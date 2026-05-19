package runner

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// PromptDef defines a prompt pattern and its response.
type PromptDef struct {
	Pattern  string
	IsRegex  bool
	Response string
	Repeat   int // how many times this prompt can match
}

// PromptResult extends Result with prompt-specific information.
type PromptResult struct {
	Result
	UnmatchedPrompts []string // patterns that were never matched
	AmbiguousMatch   string   // non-empty if two patterns matched simultaneously
	TimedOut         bool
}

// promptState tracks remaining matches for a single prompt definition.
type promptState struct {
	def       PromptDef
	regex     *regexp.Regexp
	remaining int
}

// compilePrompts prepares prompt states with compiled regexes.
func compilePrompts(prompts []PromptDef) ([]promptState, error) {
	states := make([]promptState, len(prompts))
	for i, p := range prompts {
		var re *regexp.Regexp
		if p.IsRegex {
			var err error
			re, err = regexp.Compile(p.Pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid regex %q: %w", p.Pattern, err)
			}
		}
		states[i] = promptState{def: p, regex: re, remaining: p.Repeat}
	}
	return states, nil
}

// matchPrompts finds which prompt states match the given text.
func matchPrompts(states []promptState, text string) []int {
	var matches []int
	for i, s := range states {
		if s.remaining <= 0 {
			continue
		}
		if s.regex != nil && s.regex.MatchString(text) {
			matches = append(matches, i)
		} else if s.regex == nil && strings.Contains(text, s.def.Pattern) {
			matches = append(matches, i)
		}
	}
	return matches
}

// readAndMatch reads from stdout, accumulates output, matches prompts against the
// full pending buffer, and writes responses to stdin. Closes stdin when stdout is exhausted.
func readAndMatch(stdout io.Reader, stdin io.WriteCloser, states []promptState) (stdoutContent, ambiguousErr string) {
	defer func() { _ = stdin.Close() }()

	var stdoutBuf strings.Builder
	var pending strings.Builder
	buf := make([]byte, 1024)

	for {
		n, readErr := stdout.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			stdoutBuf.WriteString(chunk)
			pending.WriteString(chunk)

			// Match against accumulated pending buffer
			matches := matchPrompts(states, pending.String())
			if len(matches) > 1 {
				return stdoutBuf.String(), fmt.Sprintf("patterns %q and %q both match: %q",
					states[matches[0]].def.Pattern, states[matches[1]].def.Pattern, pending.String())
			}
			if len(matches) == 1 {
				idx := matches[0]
				states[idx].remaining--
				_, _ = fmt.Fprintln(stdin, states[idx].def.Response)
				pending.Reset()
			}
		}
		if readErr != nil {
			return stdoutBuf.String(), ""
		}
	}
}

// collectUnmatched returns patterns that were never fully matched.
func collectUnmatched(states []promptState) []string {
	var unmatched []string
	for _, s := range states {
		if s.remaining > 0 {
			unmatched = append(unmatched, s.def.Pattern)
		}
	}
	return unmatched
}

// RunWithPrompts executes a command with interactive prompt handling.
// It reads stdout asynchronously and writes responses to stdin when patterns match.
func RunWithPrompts(command string, prompts []PromptDef, timeoutMs int) PromptResult {
	states, err := compilePrompts(prompts)
	if err != nil {
		return PromptResult{Result: Result{ExitCode: -1, Stderr: err.Error()}}
	}

	cmd := exec.Command("sh", "-c", command)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return PromptResult{Result: Result{ExitCode: -1, Stderr: fmt.Sprintf("stdin pipe error: %v", err)}}
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return PromptResult{Result: Result{ExitCode: -1, Stderr: fmt.Sprintf("stdout pipe error: %v", err)}}
	}
	stderrBuf := &syncBuffer{}
	cmd.Stderr = stderrBuf

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return PromptResult{Result: Result{ExitCode: -1, Stderr: fmt.Sprintf("start error: %v", err)}}
	}

	// Read stdout and match prompts in background
	type readResult struct {
		stdout    string
		ambiguous string
	}
	readDone := make(chan readResult, 1)
	go func() {
		stdout, ambiguous := readAndMatch(stdoutPipe, stdinPipe, states)
		readDone <- readResult{stdout, ambiguous}
	}()

	// Wait for process with timeout
	timedOut, exitCode := waitWithTimeout(cmd, timeoutMs)

	rr := <-readDone
	duration := time.Since(start).Milliseconds()

	return PromptResult{
		Result: Result{
			Stdout:     rr.stdout,
			Stderr:     stderrBuf.String(),
			ExitCode:   exitCode,
			DurationMs: duration,
		},
		UnmatchedPrompts: collectUnmatched(states),
		AmbiguousMatch:   rr.ambiguous,
		TimedOut:         timedOut,
	}
}

// waitWithTimeout waits for process exit or kills on timeout. Returns (timedOut, exitCode).
func waitWithTimeout(cmd *exec.Cmd, timeoutMs int) (timedOut bool, exitCode int) {
	timer := time.NewTimer(time.Duration(timeoutMs) * time.Millisecond)
	defer timer.Stop()

	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()

	select {
	case waitErr := <-doneCh:
		if waitErr != nil {
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				return false, exitErr.ExitCode()
			}
		}
		return false, 0
	case <-timer.C:
		_ = cmd.Process.Kill()
		<-doneCh
		return true, -1
	}
}
