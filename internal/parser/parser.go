package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sleipi/cli-t/internal/types"
)

// directive represents a parsed @directive line (parser-internal).
type directive struct {
	Name  string
	Value string
}

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
		var err error
		startLine, err = parseFrontmatter(lines, file)
		if err != nil {
			return nil, err
		}
	}

	// Parse entries
	entries, err := parseEntries(lines[startLine:])
	if err != nil {
		return nil, err
	}
	file.Entries = entries
	return file, nil
}

func parseFrontmatter(lines []string, file *types.File) (int, error) {
	var fileDirectives []directive
	i := 1
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			interpretFileDirectives(file, fileDirectives)
			return i + 1, nil
		}
		if strings.HasPrefix(line, "@") {
			d, err := parseDirective(line)
			if err != nil {
				return 0, fmt.Errorf("frontmatter line %d: %w", i+1, err)
			}
			if d != nil {
				fileDirectives = append(fileDirectives, *d)
			}
		}
		i++
	}
	return 0, fmt.Errorf("unclosed frontmatter (missing closing ---)")
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
			current.comment, i = collectComments(lines, i)
			continue
		}

		// Directive line (@group, @skip, etc.) — must be before command
		if strings.HasPrefix(strings.TrimSpace(line), "@") {
			if current == nil {
				current = &entryBuilder{}
			}
			if err := parseEntryDirective(current, line); err != nil {
				return nil, err
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
			current.command, i = collectCommand(lines, i)
			continue
		}

		// Parse post-command content (EXIT, sections, body)
		var err error
		i, err = parsePostCommand(lines, i, current)
		if err != nil {
			return nil, err
		}
	}

	flush()
	return entries, nil
}

// parsePostCommand handles lines after the command: EXIT, [Asserts], [Captures], fenced/implicit body.
func parsePostCommand(lines []string, i int, current *entryBuilder) (int, error) {
	line := lines[i]

	if strings.HasPrefix(line, "EXIT ") {
		if err := parseExitLine(current, line); err != nil {
			return 0, err
		}
		return i + 1, nil
	}

	if strings.TrimSpace(line) == "[Asserts]" {
		return collectAsserts(lines, i+1, current)
	}

	if strings.TrimSpace(line) == "[Captures]" {
		return collectCaptures(lines, i+1, current)
	}

	if strings.TrimSpace(line) == "[Prompts]" {
		return collectPrompts(lines, i+1, current)
	}

	if strings.TrimSpace(line) == "[Finally]" {
		if !current.exitNever {
			return 0, fmt.Errorf("[Finally] section is only valid on EXIT NEVER entries")
		}
		return collectFinally(lines, i+1, current)
	}

	if strings.TrimSpace(line) == "```" {
		current.body, i = collectFencedBody(lines, i+1)
		return i, nil
	}

	// Implicit body
	current.body = append(current.body, line)
	return i + 1, nil
}

func collectComments(lines []string, i int) (comment string, next int) {
	var comments []string
	for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
		comments = append(comments, strings.TrimSpace(lines[i]))
		i++
	}
	return strings.Join(comments, "\n"), i
}

func parseEntryDirective(current *entryBuilder, line string) error {
	if current.command != "" {
		return fmt.Errorf("directive must appear before command: %s", line)
	}
	d, err := parseDirective(strings.TrimSpace(line))
	if err != nil {
		return fmt.Errorf("line: %w", err)
	}
	if d != nil {
		current.directives = append(current.directives, *d)
	}
	return nil
}

func collectCommand(lines []string, i int) (cmd string, next int) {
	cmd = lines[i]
	i++
	for strings.HasSuffix(cmd, "\\") && i < len(lines) {
		cmd = cmd[:len(cmd)-1]
		cmd += lines[i]
		i++
	}
	return cmd, i
}

func parseExitLine(current *entryBuilder, line string) error {
	exitVal := strings.TrimPrefix(line, "EXIT ")
	if exitVal == "NEVER" {
		current.exitNever = true
	} else {
		code, err := strconv.Atoi(exitVal)
		if err != nil {
			return fmt.Errorf("invalid EXIT code: %s", line)
		}
		current.exitCode = code
	}
	current.hasExit = true
	return nil
}

func collectAsserts(lines []string, i int, current *entryBuilder) (int, error) {
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(lines[i], "[") {
		a, err := parseAssert(lines[i])
		if err != nil {
			return 0, fmt.Errorf("line %d: %w", i+1, err)
		}
		current.asserts = append(current.asserts, a)
		i++
	}
	return i, nil
}

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
	if len(parts) >= 5 && parts[3] == "timeout" {
		timeout, err = strconv.Atoi(parts[4])
		if err != nil {
			return nil, fmt.Errorf("invalid timeout value %q: %w", parts[4], err)
		}
	} else if len(parts) > 3 && parts[3] != "timeout" {
		return nil, fmt.Errorf("unexpected token %q after exit code (expected 'timeout')", parts[3])
	}

	return &types.Finally{
		Signal:   signal,
		ExitCode: code,
		Timeout:  timeout,
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
		// Find closing / that is not escaped (odd number of preceding backslashes)
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

func collectFencedBody(lines []string, i int) (body []string, next int) {
	for i < len(lines) && strings.TrimSpace(lines[i]) != "```" {
		body = append(body, lines[i])
		i++
	}
	i++ // skip closing ```
	return body, i
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
	prompts    []types.Prompt
	finally    *types.Finally
	directives []directive
}

func (b *entryBuilder) build() types.Entry {
	entry := types.Entry{
		Comment:   b.comment,
		Command:   b.command,
		ExitCode:  b.exitCode,
		ExitNever: b.exitNever,
		Body:      b.body,
		Asserts:   b.asserts,
		Captures:  b.captures,
		Prompts:   b.prompts,
		Finally:   b.finally,
	}
	interpretEntryDirectives(&entry, b.directives)
	return entry
}

// parseDirective parses a line like "@group BUG-1234 smoke" into a directive.
func parseDirective(line string) (*directive, error) {
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

	return &directive{Name: name, Value: value}, nil
}

// interpretFileDirectives interprets raw directives into typed FileDirectives.
func interpretFileDirectives(f *types.File, directives []directive) {
	for _, d := range directives {
		switch d.Name {
		case "group":
			if d.Value != "" {
				f.Directives.Groups = append(f.Directives.Groups, strings.Fields(d.Value)...)
			}
		case "skip":
			f.Directives.Skip = true
			f.Directives.SkipReason = d.Value
		}
	}
}

// interpretEntryDirectives interprets raw directives into typed EntryDirectives.
func interpretEntryDirectives(e *types.Entry, directives []directive) {
	for _, d := range directives {
		switch d.Name {
		case "group":
			if d.Value != "" {
				e.Directives.Groups = append(e.Directives.Groups, strings.Fields(d.Value)...)
			}
		case "skip":
			e.Directives.Skip = true
			e.Directives.SkipReason = d.Value
		case "defer":
			e.Directives.Defer = true
		case "timeout":
			if v, err := strconv.Atoi(d.Value); err == nil {
				e.Directives.Timeout = v
			}
		case "poll":
			if v, err := strconv.Atoi(d.Value); err == nil {
				e.Directives.Poll = v
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

func extractPredicate(s string) (predicate, value string) {
	p, v, _ := extractPredicateWithLater(s)
	return p, v
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
