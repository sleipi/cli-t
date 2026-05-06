package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sleipi/cli-t/internal/types"
)

// ParseFile parses a .clitest file content into a File with frontmatter and entries.
func ParseFile(input string) (*types.File, error) {
	lines := strings.Split(input, "\n")
	// Remove trailing empty line from split
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	file := &types.File{}
	startLine := 0

	// Parse frontmatter if present
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		startLine = 1
		closed := false
		for startLine < len(lines) {
			line := strings.TrimSpace(lines[startLine])
			if line == "---" {
				startLine++
				closed = true
				break
			}
			// Only parse lines starting with @ as directives; ignore prose text
			if strings.HasPrefix(line, "@") {
				d, err := parseDirective(line)
				if err != nil {
					return nil, fmt.Errorf("frontmatter line %d: %w", startLine+1, err)
				}
				if d != nil {
					file.Directives = append(file.Directives, *d)
				}
			}
			startLine++
		}
		if !closed {
			return nil, fmt.Errorf("unclosed frontmatter (missing closing ---)")
		}
	}

	// Interpret file-level directives
	interpretFileDirectives(file)

	// Parse entries
	entries, err := parseEntries(lines[startLine:])
	if err != nil {
		return nil, err
	}
	file.Entries = entries
	return file, nil
}

// Parse parses a .clitest file content into a list of entries (legacy API).
func Parse(input string) ([]types.Entry, error) {
	f, err := ParseFile(input)
	if err != nil {
		return nil, err
	}
	return f.Entries, nil
}

func parseEntries(lines []string) ([]types.Entry, error) {
	var entries []types.Entry
	var current *entryBuilder

	flush := func() {
		if current != nil {
			entries = append(entries, current.build())
			current = nil
		}
	}

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Blank line = entry separator
		if strings.TrimSpace(line) == "" {
			flush()
			i++
			continue
		}

		// Comment lines before a command attach to next entry
		if strings.HasPrefix(strings.TrimSpace(line), "#") && current == nil {
			current = &entryBuilder{}
			// Collect consecutive comment lines
			var comments []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
				comments = append(comments, strings.TrimSpace(lines[i]))
				i++
			}
			current.comment = strings.Join(comments, "\n")
			continue
		}

		// Directive line (@group, @skip, etc.) — must be before command
		if strings.HasPrefix(strings.TrimSpace(line), "@") {
			if current == nil {
				current = &entryBuilder{}
			}
			if current.command != "" {
				return nil, fmt.Errorf("directive must appear before command: %s", line)
			}
			d, err := parseDirective(strings.TrimSpace(line))
			if err != nil {
				return nil, fmt.Errorf("line: %w", err)
			}
			if d != nil {
				current.directives = append(current.directives, *d)
			}
			i++
			continue
		}

		// Start new entry if we don't have one
		if current == nil {
			current = &entryBuilder{}
		}

		// If no command yet, this line is the command
		if current.command == "" {
			cmd := line
			i++
			// Multi-line command: trailing backslash means continuation
			for strings.HasSuffix(cmd, "\\") && i < len(lines) {
				cmd = cmd[:len(cmd)-1] // strip trailing backslash
				cmd += lines[i]
				i++
			}
			current.command = cmd
			continue
		}

		// EXIT line
		if strings.HasPrefix(line, "EXIT ") {
			exitVal := strings.TrimPrefix(line, "EXIT ")
			if exitVal == "NEVER" {
				current.exitNever = true
			} else {
				code, err := strconv.Atoi(exitVal)
				if err != nil {
					return nil, fmt.Errorf("invalid EXIT code: %s", line)
				}
				current.exitCode = code
			}
			current.hasExit = true
			i++
			continue
		}

		// [Asserts] section
		if strings.TrimSpace(line) == "[Asserts]" {
			i++
			for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(lines[i], "[") {
				a, err := parseAssert(lines[i])
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", i+1, err)
				}
				current.asserts = append(current.asserts, a)
				i++
			}
			continue
		}

		// [Captures] section
		if strings.TrimSpace(line) == "[Captures]" {
			i++
			for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(lines[i], "[") {
				c, err := parseCapture(lines[i])
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", i+1, err)
				}
				current.captures = append(current.captures, c)
				i++
			}
			continue
		}

		// Fenced body (```)
		if strings.TrimSpace(line) == "```" {
			i++
			for i < len(lines) && strings.TrimSpace(lines[i]) != "```" {
				current.body = append(current.body, lines[i])
				i++
			}
			i++ // skip closing ```
			continue
		}

		// Otherwise it's implicit body
		current.body = append(current.body, line)
		i++
	}

	flush()
	return entries, nil
}

type entryBuilder struct {
	comment    string
	command    string
	exitCode   int
	exitNever  bool
	hasExit    bool
	body       []string
	asserts    []types.Assert
	captures   []types.Capture
	directives []types.Directive
}

func (b *entryBuilder) build() types.Entry {
	entry := types.Entry{
		Comment:    b.comment,
		Command:    b.command,
		ExitCode:   b.exitCode,
		ExitNever:  b.exitNever,
		Body:       b.body,
		Asserts:    b.asserts,
		Captures:   b.captures,
		Directives: b.directives,
	}
	interpretEntryDirectives(&entry)
	return entry
}

// parseDirective parses a line like "@group BUG-1234 smoke" into a Directive.
func parseDirective(line string) (*types.Directive, error) {
	if !strings.HasPrefix(line, "@") {
		return nil, fmt.Errorf("not a directive: %s", line)
	}

	// Split into @name and value
	parts := strings.SplitN(line, " ", 2)
	name := strings.TrimPrefix(parts[0], "@")
	if name == "" {
		return nil, fmt.Errorf("empty directive name: %s", line)
	}

	value := ""
	if len(parts) == 2 {
		value = strings.TrimSpace(parts[1])
	}

	return &types.Directive{Name: name, Value: value}, nil
}

// interpretFileDirectives interprets raw directives into typed File fields.
func interpretFileDirectives(f *types.File) {
	for _, d := range f.Directives {
		switch d.Name {
		case "group":
			if d.Value != "" {
				f.Groups = append(f.Groups, strings.Fields(d.Value)...)
			}
		case "skip":
			f.Skip = true
			f.SkipReason = d.Value
		}
	}
}

// interpretEntryDirectives interprets raw directives into typed Entry fields.
func interpretEntryDirectives(e *types.Entry) {
	for _, d := range e.Directives {
		switch d.Name {
		case "group":
			if d.Value != "" {
				e.Groups = append(e.Groups, strings.Fields(d.Value)...)
			}
		case "skip":
			e.Skip = true
			e.SkipReason = d.Value
		case "defer":
			e.Defer = true
		case "timeout":
			if v, err := strconv.Atoi(d.Value); err == nil {
				e.Timeout = v
			}
		case "poll":
			if v, err := strconv.Atoi(d.Value); err == nil {
				e.Poll = v
			}
		}
	}
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

	// Extract predicate and value
	predicate, value := extractPredicate(rest)

	return types.Assert{
		Query:     query,
		Predicate: predicate,
		Value:     value,
		Negated:   negated,
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

func extractPredicate(s string) (predicate, value string) {
	// Predicates without value
	noValuePredicates := []string{"isEmpty"}
	for _, p := range noValuePredicates {
		if s == p {
			return p, ""
		}
	}

	// Predicates with value
	predicates := []string{"contains", "not contains", "startsWith", "endsWith", "matches", "==", "!=", ">=", "<=", ">", "<"}
	for _, p := range predicates {
		if strings.HasPrefix(s, p+" ") || s == p {
			val := strings.TrimSpace(strings.TrimPrefix(s, p))
			val = unquoteValue(val)
			return p, val
		}
	}

	return s, ""
}

func unquoteValue(s string) string {
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
