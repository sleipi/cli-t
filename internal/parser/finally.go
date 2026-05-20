package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sleipi/cli-t/internal/types"
)

const keywordTimeout = "timeout"

// collectFinally parses a [Finally] section for EXIT NEVER entries.
// First line: <SIGNAL> EXIT <code> [timeout <ms>]
// Subsequent lines: asserts (until blank line or next section).
func collectFinally(lines []string, i int, current *entryBuilder) (int, error) {
	if i >= len(lines) || strings.TrimSpace(lines[i]) == "" {
		return 0, fmt.Errorf("[Finally] section requires a signal line")
	}

	fin, err := parseFinallySignalLine(lines[i])
	if err != nil {
		return 0, fmt.Errorf("line %d: %w", i+1, err)
	}
	i++

	// Collect optional asserts
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(lines[i], "[") {
		a, err := parseAssert(lines[i])
		if err != nil {
			return 0, fmt.Errorf("line %d: %w", i+1, err)
		}
		fin.Asserts = append(fin.Asserts, a)
		i++
	}

	current.finally = fin
	return i, nil
}

// parseFinallySignalLine parses: <SIGNAL> EXIT <code> [timeout <ms>]
func parseFinallySignalLine(line string) (*types.Finally, error) {
	line = strings.TrimSpace(line)
	parts := strings.Fields(line)

	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid [Finally] signal line: %s (expected: SIGNAL EXIT CODE [timeout MS])", line)
	}

	signal := parts[0]
	validSignals := map[string]bool{"TERM": true, "KILL": true, "INT": true, "HUP": true, "QUIT": true}
	if !validSignals[signal] {
		return nil, fmt.Errorf("unsupported signal %q (supported: TERM, KILL, INT, HUP, QUIT)", signal)
	}

	if parts[1] != "EXIT" {
		return nil, fmt.Errorf("expected EXIT after signal name, got %q", parts[1])
	}

	code, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid exit code %q: %w", parts[2], err)
	}

	timeout := 1000 // default 1000ms
	if len(parts) >= 5 && parts[3] == keywordTimeout {
		timeout, err = strconv.Atoi(parts[4])
		if err != nil {
			return nil, fmt.Errorf("invalid timeout value %q: %w", parts[4], err)
		}
	} else if len(parts) > 3 && parts[3] != keywordTimeout {
		return nil, fmt.Errorf("unexpected token %q after exit code (expected 'timeout')", parts[3])
	}

	return &types.Finally{
		Signal:   signal,
		ExitCode: code,
		Timeout:  timeout,
	}, nil
}
