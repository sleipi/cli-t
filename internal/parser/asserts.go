package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sleipi/cli-t/internal/types"
)

// collectAsserts collects assert lines from an [asserts] section.
func collectAsserts(lines []string, i int, current *entryBuilder) (int, error) {
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(lines[i], "[") {
		a, nextIdx, isMultiline, err := tryParseMultilineAssert(lines, i)
		if err != nil {
			return 0, fmt.Errorf("line %d: %w", i+1, err)
		}
		if isMultiline {
			current.asserts = append(current.asserts, a)
			i = nextIdx
			continue
		}

		a2, err := parseAssert(lines[i])
		if err != nil {
			return 0, fmt.Errorf("line %d: %w", i+1, err)
		}
		current.asserts = append(current.asserts, a2)
		i++
	}
	return i, nil
}

const tripleQuote = `"""`
const predicateMatches = "matches"

// tryParseMultilineAssert attempts to parse a multi-line assert starting at lines[i].
// Returns (assert, nextIndex, wasMultiline, error).
func tryParseMultilineAssert(lines []string, i int) (assert types.Assert, nextIndex int, wasMultiline bool, err error) {
	line := strings.TrimSpace(lines[i])

	// Check if line ends with """
	if !strings.HasSuffix(line, tripleQuote) {
		return types.Assert{}, 0, false, nil
	}

	// Check for "later" modifier at the start of the line
	later := false
	if strings.HasPrefix(line, "later ") {
		later = true
		line = strings.TrimSpace(strings.TrimPrefix(line, "later "))
	}

	// Extract query
	query, rest := extractQuery(line)
	if query == "" {
		return types.Assert{}, 0, false, fmt.Errorf("cannot parse assert: %s", line)
	}
	rest = strings.TrimSpace(rest)

	// Check negation
	negated := false
	if strings.HasPrefix(rest, "not ") {
		negated = true
		rest = strings.TrimPrefix(rest, "not ")
		rest = strings.TrimSpace(rest)
	}

	// Extract predicate (everything before the """)
	predicate, valuePart, _ := extractPredicate(rest)

	// The value part should be exactly """ (nothing else after opening)
	if valuePart != tripleQuote {
		return types.Assert{}, 0, false, nil
	}

	// Reject matches with triple-quote
	if predicate == predicateMatches {
		return types.Assert{}, 0, false, fmt.Errorf("multi-line triple-quote values are not supported with 'matches' predicate")
	}

	// Consume lines until closing """
	var contentLines []string
	j := i + 1
	for j < len(lines) {
		if strings.TrimSpace(lines[j]) == tripleQuote {
			// Found closing """
			value := strings.Join(contentLines, "\n")
			return types.Assert{
				Query:     query,
				Predicate: predicate,
				Value:     value,
				Negated:   negated,
				Later:     later,
			}, j + 1, true, nil
		}
		contentLines = append(contentLines, lines[j])
		j++
	}

	return types.Assert{}, 0, false, fmt.Errorf("unterminated triple-quote block (opening \"\"\" without closing \"\"\")")
}

// collectCaptures collects capture lines from a [captures] section.
func collectCaptures(lines []string, i int, current *entryBuilder) (int, error) {
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(lines[i], "[") {
		c, err := parseCapture(lines[i])
		if err != nil {
			return 0, fmt.Errorf("line %d: %w", i+1, err)
		}
		current.captures = append(current.captures, c)
		i++
	}
	return i, nil
}

// collectPrompts collects prompt lines from a [prompts] section.
func collectPrompts(lines []string, i int, current *entryBuilder) (int, error) {
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(lines[i], "[") {
		p, err := parsePrompt(lines[i])
		if err != nil {
			return 0, fmt.Errorf("line %d: %w", i+1, err)
		}
		current.prompts = append(current.prompts, p)
		i++
	}
	return i, nil
}

// parseAssert parses an assert line.
// Syntax: [later] <query> [not] <predicate> [<value>]
// e.g. "later stdout contains \"hello\"" or "stdout not contains \"error\""
func parseAssert(line string) (types.Assert, error) {
	line = strings.TrimSpace(line)

	// Check for "later" modifier at the start of the line
	later := false
	if strings.HasPrefix(line, "later ") {
		later = true
		line = strings.TrimSpace(strings.TrimPrefix(line, "later "))
	}

	// Extract query
	query, rest := extractQuery(line)
	if query == "" {
		return types.Assert{}, fmt.Errorf("cannot parse assert: %s", line)
	}

	rest = strings.TrimSpace(rest)

	// Check negation
	negated := false
	if strings.HasPrefix(rest, "not ") {
		negated = true
		rest = strings.TrimPrefix(rest, "not ")
		rest = strings.TrimSpace(rest)
	}

	// Extract predicate and value
	predicate, value, _ := extractPredicate(rest)

	// Validate regex at parse time
	if predicate == predicateMatches && value != "" {
		if _, err := regexp.Compile(value); err != nil {
			return types.Assert{}, fmt.Errorf("invalid regex in assert: %w", err)
		}
	}

	return types.Assert{
		Query:     query,
		Predicate: predicate,
		Value:     value,
		Negated:   negated,
		Later:     later,
	}, nil
}

func extractQuery(line string) (query, rest string) {
	// "line N" query
	if strings.HasPrefix(line, "line ") {
		parts := strings.SplitN(line, " ", 3)
		if len(parts) >= 3 {
			return parts[0] + " " + parts[1], parts[2]
		}
	}

	// Known single-word queries
	knownQueries := []string{"stdout", "stderr", "lineCount", "duration", "exit"}
	for _, q := range knownQueries {
		if strings.HasPrefix(line, q+" ") || line == q {
			return q, strings.TrimPrefix(line, q)
		}
	}

	// Fallback: first word
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

// extractPredicate parses predicate and value from the remainder of an assert line
// (after query and optional "not" have been removed).
func extractPredicate(s string) (predicate, value string, later bool) {
	// Predicates without value
	noValuePredicates := []string{"isEmpty"}
	for _, p := range noValuePredicates {
		if s == p {
			return p, "", false
		}
	}

	// Predicates with value.
	// "not" is handled upstream in parseAssert — "not contains" here is not needed.
	// Bare-value semantics: everything after the predicate is the value;
	// quotes are optional but stripped if present ("hello" → hello, /pat/ → pat).
	predicates := []string{"contains", "startsWith", "endsWith", predicateMatches, "==", "!=", ">=", "<=", ">", "<"}
	for _, p := range predicates {
		if strings.HasPrefix(s, p+" ") || s == p {
			val := strings.TrimSpace(strings.TrimPrefix(s, p))
			val = unquoteValue(val)
			return p, val, false
		}
	}

	return s, "", false
}

func unquoteValue(s string) string {
	// Triple-quote marker — return as-is (handled by multi-line parser)
	if s == tripleQuote {
		return s
	}
	// Quoted string "..."
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	// Regex literal /pattern/
	if len(s) >= 2 && s[0] == '/' && s[len(s)-1] == '/' {
		return s[1 : len(s)-1]
	}
	return s
}

func parseCapture(line string) (types.Capture, error) {
	line = strings.TrimSpace(line)
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		if strings.Contains(line, ":") {
			return types.Capture{}, fmt.Errorf("invalid capture syntax: use '=' instead of ':' (e.g. id = stdout)")
		}
		return types.Capture{}, fmt.Errorf("invalid capture: %s (expected: name = query)", line)
	}
	name := strings.TrimSpace(parts[0])
	query := strings.TrimSpace(parts[1])

	switch {
	case query == "stdout":
		return types.Capture{Name: name, Source: types.CaptureStdout}, nil
	case query == "stderr":
		return types.Capture{Name: name, Source: types.CaptureStderr}, nil
	case query == "lineCount":
		return types.Capture{Name: name, Source: types.CaptureLineCount}, nil
	case query == "pid":
		return types.Capture{Name: name, Source: types.CapturePid}, nil
	case strings.HasPrefix(query, "line "):
		numStr := strings.TrimPrefix(query, "line ")
		n, err := strconv.Atoi(numStr)
		if err != nil || n < 1 {
			return types.Capture{}, fmt.Errorf("invalid line number in capture: %s", query)
		}
		return types.Capture{Name: name, Source: types.CaptureLine, LineNum: n}, nil
	case strings.HasPrefix(query, "stdout regex "), strings.HasPrefix(query, "stderr regex "):
		var source types.CaptureSource
		var rest string
		if strings.HasPrefix(query, "stdout regex ") {
			source = types.CaptureStdout
			rest = strings.TrimPrefix(query, "stdout regex ")
		} else {
			source = types.CaptureStderr
			rest = strings.TrimPrefix(query, "stderr regex ")
		}
		pattern := unquoteValue(strings.TrimSpace(rest))
		re, err := regexp.Compile(pattern)
		if err != nil {
			return types.Capture{}, fmt.Errorf("invalid regex in capture: %w", err)
		}
		return types.Capture{Name: name, Source: source, Regex: re}, nil
	default:
		return types.Capture{}, fmt.Errorf("unknown capture query: %s", query)
	}
}

func parsePrompt(line string) (types.Prompt, error) {
	line = strings.TrimSpace(line)
	var pattern string
	var isRegex bool
	var rest string

	switch {
	case strings.HasPrefix(line, "/"):
		// Regex pattern: /pattern/ => "response"
		end := -1
		for j := 1; j < len(line); j++ {
			if line[j] != '/' {
				continue
			}
			backslashes := 0
			for k := j - 1; k >= 1 && line[k] == '\\'; k-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				end = j
				break
			}
		}
		if end == -1 {
			return types.Prompt{}, fmt.Errorf("unterminated regex pattern: %s", line)
		}
		pattern = line[1:end]
		isRegex = true
		rest = strings.TrimSpace(line[end+1:])
	case strings.HasPrefix(line, `"`):
		// Quoted pattern: "pattern" => "response"
		end := strings.Index(line[1:], `"`)
		if end == -1 {
			return types.Prompt{}, fmt.Errorf("unterminated quoted pattern: %s", line)
		}
		pattern = line[1 : end+1]
		rest = strings.TrimSpace(line[end+2:])
	default:
		return types.Prompt{}, fmt.Errorf("prompt pattern must be quoted or regex: %s", line)
	}

	// Expect =>
	if !strings.HasPrefix(rest, "=>") {
		return types.Prompt{}, fmt.Errorf("expected '=>' after pattern: %s", line)
	}
	rest = strings.TrimSpace(rest[2:])

	// Parse response: "response"
	if !strings.HasPrefix(rest, `"`) {
		return types.Prompt{}, fmt.Errorf("response must be quoted: %s", line)
	}
	endQuote := strings.Index(rest[1:], `"`)
	if endQuote == -1 {
		return types.Prompt{}, fmt.Errorf("unterminated response: %s", line)
	}
	response := rest[1 : endQuote+1]
	rest = strings.TrimSpace(rest[endQuote+2:])

	// Parse optional multiplier: * N
	repeat := 1
	if strings.HasPrefix(rest, "*") {
		rest = strings.TrimSpace(rest[1:])
		n, err := strconv.Atoi(rest)
		if err != nil {
			return types.Prompt{}, fmt.Errorf("invalid multiplier: %s", line)
		}
		repeat = n
	}

	return types.Prompt{
		Pattern:  pattern,
		IsRegex:  isRegex,
		Response: response,
		Repeat:   repeat,
	}, nil
}
