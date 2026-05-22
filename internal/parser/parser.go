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
