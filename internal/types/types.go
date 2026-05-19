package types

// EntryDirectives holds interpreted directives for an entry.
type EntryDirectives struct {
	Groups     []string
	Skip       bool
	SkipReason string
	Defer      bool
	Timeout    int // @timeout in ms (0 = not set)
	Poll       int // @poll in ms (0 = default 100ms)
}

// FileDirectives holds interpreted directives for a file.
type FileDirectives struct {
	Groups     []string
	Skip       bool
	SkipReason string
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
	Directives EntryDirectives
}

// Assert represents a single explicit assertion.
type Assert struct {
	Query     string // e.g. "stdout", "stderr", "line 1", "lineCount", "duration"
	Predicate string // e.g. "contains", "==", "matches", "isEmpty", "startsWith"
	Value     string // predicate value (empty for isEmpty)
	Negated   bool   // "not contains" etc.
}

// Capture represents a variable capture from command output.
type Capture struct {
	Name  string // variable name
	Query string // e.g. "stdout", "line 1"
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
