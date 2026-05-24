package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sleipi/cli-t/internal/types"
)

// collectAsserts collects assert lines from an [Asserts] section.
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

// tryParseMultilineAssert attempts to parse a multi-line assert starting at lines[i].
// Returns (assert, nextIndex, wasMultiline, error).
func tryParseMultilineAssert(lines []string, i int) (assert types.Assert, nextIndex int, wasMultiline bool, err error) {
	line := strings.TrimSpace(lines[i])

	// Check if line ends with """
	if !strings.HasSuffix(line, tripleQuote) {
		return types.Assert{}, 0, false, nil
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
	predicate, valuePart, later := extractPredicateWithLater(rest)

	// The value part should be exactly """ (nothing else after opening)
	if valuePart != tripleQuote {
		return types.Assert{}, 0, false, nil
	}

	// Reject matches with triple-quote
	if predicate == "matches" {
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

// collectCaptures collects capture lines from a [Captures] section.
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

// collectPrompts collects prompt lines from a [Prompts] section.
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

// parseAssert parses a line like: stdout contains "hello"
func parseAssert(line string) (types.Assert, error) {
	line = strings.TrimSpace(line)

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

	// Extract predicate and value (with possible "later" modifier)
	predicate, value, later := extractPredicateWithLater(rest)

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

// extractPredicateWithLater parses predicate, optional "later" modifier, and value.
// Syntax: <predicate> [later] <value>
// e.g. "contains later \"hello\"" → ("contains", "hello", true)
func extractPredicateWithLater(s string) (predicate, value string, later bool) {
	// Predicates without value
	noValuePredicates := []string{"isEmpty"}
	for _, p := range noValuePredicates {
		if s == p {
			return p, "", false
		}
	}

	// Predicates with value
	predicates := []string{"contains", "not contains", "startsWith", "endsWith", "matches", "==", "!=", ">=", "<=", ">", "<"}
	for _, p := range predicates {
		if strings.HasPrefix(s, p+" ") || s == p {
			val := strings.TrimSpace(strings.TrimPrefix(s, p))
			// Check for "later" modifier before value
			if strings.HasPrefix(val, "later ") {
				later = true
				val = strings.TrimSpace(strings.TrimPrefix(val, "later"))
			}
			val = unquoteValue(val)
			return p, val, later
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
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return types.Capture{}, fmt.Errorf("invalid capture: %s", line)
	}
	return types.Capture{
		Name:  strings.TrimSpace(parts[0]),
		Query: strings.TrimSpace(parts[1]),
	}, nil
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
