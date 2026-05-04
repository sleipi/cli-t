package types

// Entry represents a single test block in a .clit file.
type Entry struct {
	Comment  string   // optional comment/description
	Command  string   // the shell command to execute
	ExitCode int      // expected exit code
	Body     []string // expected stdout lines (implicit assert, exact match)
	Asserts  []Assert // explicit [Asserts] section
	Captures []Capture
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

// File represents a parsed .clit file.
type File struct {
	Path    string
	Entries []Entry
}
