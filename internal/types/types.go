package types

import "regexp"

// EntryDirectives holds interpreted directives for an entry.
type EntryDirectives struct {
	Groups     []string
	Skip       bool
	SkipReason string
	Defer      bool
	Timeout    *int              // @timeout in ms (nil = not set, 0 = no timeout)
	Poll       *int              // @poll in ms (nil = not set, use default)
	Workdir    string            // @workdir path (empty = not set)
	Env        map[string]string // @env KEY=VALUE (nil = not set)
}

// FileDirectives holds interpreted directives for a file.
type FileDirectives struct {
	Groups     []string
	Skip       bool
	SkipReason string
	Timeout    *int              // @timeout in ms (nil = not set, 0 = no timeout)
	Workdir    string            // @workdir path (empty = not set)
	Env        map[string]string // @env KEY=VALUE (nil = not set)
}

// Finally represents a [Finally] section for EXIT NEVER entries.
// It sends a signal at file-end and asserts exit behavior.
type Finally struct {
	Signal   string // signal name: TERM, KILL, INT, HUP, QUIT
	ExitCode int    // expected exit code after signal
	Timeout  int    // ms to wait for process exit (default 1000)
	Asserts  []Assert
}

// Entry represents a single test block in a .clitest file.
type Entry struct {
	Comment    string
	Command    string
	ExitCode   int
	ExitNever  bool
	Body       []string
	Asserts    []Assert
	Captures   []Capture
	Prompts    []Prompt
	Finally    *Finally // only valid on ExitNever entries
	Directives EntryDirectives
}

// Assert represents a single explicit assertion.
type Assert struct {
	Query     string // e.g. "stdout", "stderr", "line 1", "lineCount", "duration"
	Predicate string // e.g. "contains", "==", "matches", "isEmpty", "startsWith"
	Value     string // predicate value (empty for isEmpty)
	Negated   bool   // "not contains" etc.
	Later     bool   // if true, evaluated at file-end instead of during polling
}

// CaptureSource identifies what a capture reads from.
type CaptureSource string

const (
	CaptureStdout    CaptureSource = "stdout"
	CaptureStderr    CaptureSource = "stderr"
	CaptureLine      CaptureSource = "line"
	CaptureLineCount CaptureSource = "lineCount"
	CapturePid       CaptureSource = "pid"
)

// Capture represents a variable capture from command output.
type Capture struct {
	Name    string         // variable name
	Source  CaptureSource  // what to read from
	LineNum int            // 1-indexed, only meaningful when Source == CaptureLine
	Regex   *regexp.Regexp // compiled pattern, nil unless regex capture
}

// Prompt represents an interactive prompt/response pair.
type Prompt struct {
	Pattern  string // substring or regex content (without delimiters)
	IsRegex  bool   // true if pattern was /regex/, false if "substring"
	Response string // response to write to stdin (without trailing newline)
	Repeat   int    // number of times this prompt can match (default 1)
}

// File represents a parsed .clitest file.
type File struct {
	Path       string
	Entries    []Entry
	Directives FileDirectives
}
