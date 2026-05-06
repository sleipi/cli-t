package types

// Directive represents a parsed @directive line (generic).
type Directive struct {
	Name  string // e.g. "group", "skip"
	Value string // raw string after @name (may be empty)
}

// Entry represents a single test block in a .clitest file.
type Entry struct {
	Comment    string      // optional comment/description
	Command    string      // the shell command to execute
	ExitCode   int         // expected exit code
	Body       []string    // expected stdout lines (implicit assert, exact match)
	Asserts    []Assert    // explicit [Asserts] section
	Captures   []Capture
	Directives []Directive // raw parsed directives
	Groups     []string    // interpreted from @group
	Skip       bool        // interpreted from @skip
	SkipReason string      // optional reason from @skip
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

// File represents a parsed .clitest file.
type File struct {
	Path       string
	Entries    []Entry
	Directives []Directive // file-level from frontmatter
	Groups     []string    // file-level groups
	Skip       bool        // file-level skip
	SkipReason string      // file-level skip reason
}
