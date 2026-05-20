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

// parseEntryDirective parses and appends a directive to an entry builder.
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

// parseFrontmatter parses the frontmatter block between --- delimiters.
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
