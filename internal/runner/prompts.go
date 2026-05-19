package runner

import (
	"errors"
	"fmt"
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

// RunWithPrompts executes a command with interactive prompt handling.
// It reads stdout asynchronously and writes responses to stdin when patterns match.
func RunWithPrompts(command string, prompts []PromptDef, timeoutMs int) PromptResult {
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

	// Compile regex patterns
	type promptState struct {
		def       PromptDef
		regex     *regexp.Regexp
		remaining int
	}
	states := make([]promptState, len(prompts))
	for i, p := range prompts {
		var re *regexp.Regexp
		if p.IsRegex {
			re, err = regexp.Compile(p.Pattern)
			if err != nil {
				return PromptResult{Result: Result{ExitCode: -1, Stderr: fmt.Sprintf("invalid regex %q: %v", p.Pattern, err)}}
			}
		}
		states[i] = promptState{def: p, regex: re, remaining: p.Repeat}
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return PromptResult{Result: Result{ExitCode: -1, Stderr: fmt.Sprintf("start error: %v", err)}}
	}

	// Channels for coordination
	stdoutDone := make(chan struct{})
	var stdoutBuf strings.Builder
	var ambiguous string

	// Read stdout and match prompts
	go func() {
		defer close(stdoutDone)
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				stdoutBuf.WriteString(chunk)

				// Check for matches against accumulated unprocessed output
				var matches []int
				for i, s := range states {
					if s.remaining <= 0 {
						continue
					}
					if s.regex != nil {
						if s.regex.MatchString(chunk) {
							matches = append(matches, i)
						}
					} else {
						if strings.Contains(chunk, s.def.Pattern) {
							matches = append(matches, i)
						}
					}
				}

				if len(matches) > 1 {
					ambiguous = fmt.Sprintf("patterns %q and %q both match: %q",
						states[matches[0]].def.Pattern, states[matches[1]].def.Pattern, chunk)
					return
				}

				if len(matches) == 1 {
					idx := matches[0]
					states[idx].remaining--
					_, _ = fmt.Fprintln(stdinPipe, states[idx].def.Response)
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Wait with timeout
	timeout := time.Duration(timeoutMs) * time.Millisecond
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.Wait()
	}()

	var timedOut bool
	var exitCode int

	select {
	case waitErr := <-doneCh:
		<-stdoutDone
		if waitErr != nil {
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			}
		}
	case <-timer.C:
		timedOut = true
		_ = cmd.Process.Kill()
		<-doneCh
		<-stdoutDone
	}

	duration := time.Since(start).Milliseconds()

	// Collect unmatched prompts
	var unmatched []string
	for _, s := range states {
		if s.remaining > 0 {
			unmatched = append(unmatched, s.def.Pattern)
		}
	}

	return PromptResult{
		Result: Result{
			Stdout:     stdoutBuf.String(),
			Stderr:     stderrBuf.String(),
			ExitCode:   exitCode,
			DurationMs: duration,
		},
		UnmatchedPrompts: unmatched,
		AmbiguousMatch:   ambiguous,
		TimedOut:         timedOut,
	}
}
