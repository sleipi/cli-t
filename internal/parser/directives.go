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
	Line  int // 1-based line number in the file
}

// DirectiveError represents a validation error for a malformed directive.
type DirectiveError struct {
	Line      int
	Directive string
	Message   string
}

func (e *DirectiveError) Error() string {
	return fmt.Sprintf("line %d: @%s: %s", e.Line, e.Directive, e.Message)
}

// parseDirective parses a line like "@group BUG-1234 smoke" into a directive.
// lineNum is the 1-based line number in the file.
func parseDirective(line string, lineNum int) (*directive, error) {
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

	return &directive{Name: name, Value: value, Line: lineNum}, nil
}

// parseEntryDirective parses and appends a directive to an entry builder.
// lineNum is the 1-based line number in the file.
func parseEntryDirective(current *entryBuilder, line string, lineNum int) error {
	if current.command != "" {
		return fmt.Errorf("directive must appear before command: %s", line)
	}
	d, err := parseDirective(strings.TrimSpace(line), lineNum)
	if err != nil {
		return fmt.Errorf("line %d: %w", lineNum, err)
	}
	if d != nil {
		current.directives = append(current.directives, *d)
	}
	return nil
}

// parseFrontmatter parses the frontmatter block between --- delimiters.
func parseFrontmatter(lines []string, file *types.File) (int, []error) {
	var fileDirectives []directive
	i := 1
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			errs := interpretFileDirectives(file, fileDirectives)
			return i + 1, errs
		}
		if strings.HasPrefix(line, "@") {
			d, err := parseDirective(line, i+1)
			if err != nil {
				return 0, []error{fmt.Errorf("frontmatter line %d: %w", i+1, err)}
			}
			if d != nil {
				fileDirectives = append(fileDirectives, *d)
			}
		}
		i++
	}
	return 0, []error{fmt.Errorf("unclosed frontmatter (missing closing ---)")}
}

// interpretFileDirectives interprets raw directives into typed FileDirectives.
// Returns validation errors for malformed directive values.
func interpretFileDirectives(f *types.File, directives []directive) []error {
	var errs []error
	for _, d := range directives {
		switch d.Name {
		case "group":
			if d.Value != "" {
				f.Directives.Groups = append(f.Directives.Groups, strings.Fields(d.Value)...)
			}
		case "skip":
			f.Directives.Skip = true
			f.Directives.SkipReason = d.Value
		case "workdir":
			f.Directives.Workdir = d.Value
		case "timeout":
			v, err := strconv.Atoi(d.Value)
			if err != nil || v < 0 {
				errs = append(errs, &DirectiveError{Line: d.Line, Directive: d.Name, Message: fmt.Sprintf("value must be a non-negative integer (milliseconds), got %q", d.Value)})
			} else {
				f.Directives.Timeout = &v
			}
		case "env":
			parts := strings.SplitN(d.Value, "=", 2)
			if len(parts) < 2 || parts[0] == "" {
				errs = append(errs, &DirectiveError{Line: d.Line, Directive: d.Name, Message: fmt.Sprintf("value must be KEY=VALUE with non-empty key, got %q", d.Value)})
			} else {
				if f.Directives.Env == nil {
					f.Directives.Env = make(map[string]string)
				}
				f.Directives.Env[parts[0]] = parts[1]
			}
		}
	}
	return errs
}

// interpretEntryDirectives interprets raw directives into typed EntryDirectives.
// Returns validation errors for malformed directive values.
func interpretEntryDirectives(e *types.Entry, directives []directive) []error {
	var errs []error
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
			v, err := strconv.Atoi(d.Value)
			if err != nil || v < 0 {
				errs = append(errs, &DirectiveError{Line: d.Line, Directive: d.Name, Message: fmt.Sprintf("value must be a non-negative integer (milliseconds), got %q", d.Value)})
			} else {
				e.Directives.Timeout = &v
			}
		case "poll":
			v, err := strconv.Atoi(d.Value)
			if err != nil || v <= 0 {
				errs = append(errs, &DirectiveError{Line: d.Line, Directive: d.Name, Message: fmt.Sprintf("value must be a positive integer (milliseconds), got %q", d.Value)})
			} else {
				e.Directives.Poll = &v
			}
		case "workdir":
			e.Directives.Workdir = d.Value
		case "env":
			parts := strings.SplitN(d.Value, "=", 2)
			if len(parts) < 2 || parts[0] == "" {
				errs = append(errs, &DirectiveError{Line: d.Line, Directive: d.Name, Message: fmt.Sprintf("value must be KEY=VALUE with non-empty key, got %q", d.Value)})
			} else {
				if e.Directives.Env == nil {
					e.Directives.Env = make(map[string]string)
				}
				e.Directives.Env[parts[0]] = parts[1]
			}
		}
	}
	return errs
}
