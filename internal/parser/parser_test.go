package parser

import (
	"testing"

	"github.com/sleipi/clit/internal/types"
)

func TestParseSimpleEcho(t *testing.T) {
	input := `# Test echo
echo "hello"
EXIT 0
hello
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	assertEqual(t, e.Command, `echo "hello"`)
	assertEqual(t, e.Comment, "# Test echo")
	assertIntEqual(t, e.ExitCode, 0)
	if len(e.Body) != 1 || e.Body[0] != "hello" {
		t.Fatalf("expected body [hello], got %v", e.Body)
	}
}

func TestParseMultipleEntries(t *testing.T) {
	input := `echo "first"
EXIT 0
first

echo "second"
EXIT 0
second
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	assertEqual(t, entries[0].Command, `echo "first"`)
	assertEqual(t, entries[1].Command, `echo "second"`)
}

func TestParseWithAsserts(t *testing.T) {
	input := `grep "beer" drinks.log
EXIT 0
[Asserts]
line 1 contains "cold beer"
stdout matches /\d+ beers/
stderr isEmpty
lineCount == 4
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	assertEqual(t, e.Command, `grep "beer" drinks.log`)
	if len(e.Asserts) != 4 {
		t.Fatalf("expected 4 asserts, got %d: %+v", len(e.Asserts), e.Asserts)
	}

	assertAssert(t, e.Asserts[0], types.Assert{Query: "line 1", Predicate: "contains", Value: "cold beer"})
	assertAssert(t, e.Asserts[1], types.Assert{Query: "stdout", Predicate: "matches", Value: `\d+ beers`})
	assertAssert(t, e.Asserts[2], types.Assert{Query: "stderr", Predicate: "isEmpty", Value: ""})
	assertAssert(t, e.Asserts[3], types.Assert{Query: "lineCount", Predicate: "==", Value: "4"})
}

func TestParseNegatedPredicate(t *testing.T) {
	input := `echo "hello"
EXIT 0
[Asserts]
stdout not contains "error"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := entries[0].Asserts[0]
	if !a.Negated {
		t.Fatal("expected negated assert")
	}
	assertEqual(t, a.Predicate, "contains")
	assertEqual(t, a.Value, "error")
}

func TestParseExitCodeNonZero(t *testing.T) {
	input := `cat nonexistent
EXIT 1
[Asserts]
stderr contains "No such file"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertIntEqual(t, entries[0].ExitCode, 1)
}

func TestParseWithCaptures(t *testing.T) {
	input := `cat /tmp/app.pid
EXIT 0
[Captures]
pid: stdout
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries[0].Captures) != 1 {
		t.Fatalf("expected 1 capture, got %d", len(entries[0].Captures))
	}
	assertEqual(t, entries[0].Captures[0].Name, "pid")
	assertEqual(t, entries[0].Captures[0].Query, "stdout")
}

func TestParseFencedBody(t *testing.T) {
	input := "printf \"a\\n\\nb\"\nEXIT 0\n```\na\n\nb\n```\n"
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries[0].Body) != 3 {
		t.Fatalf("expected 3 body lines, got %d: %v", len(entries[0].Body), entries[0].Body)
	}
	assertEqual(t, entries[0].Body[0], "a")
	assertEqual(t, entries[0].Body[1], "")
	assertEqual(t, entries[0].Body[2], "b")
}

func TestParseMultilineCommand(t *testing.T) {
	input := `# Multi-line curl
curl -s https://example.com \
  -H "Accept: application/json"
EXIT 0
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	assertEqual(t, entries[0].Command, `curl -s https://example.com   -H "Accept: application/json"`)
}

func TestParseMultilineCommandMultipleContinuations(t *testing.T) {
	input := `echo \
hello \
world
EXIT 0
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, entries[0].Command, "echo hello world")
}

func TestParseEmbeddedBackslashNotContinuation(t *testing.T) {
	input := `echo "hello\nworld"
EXIT 0
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, entries[0].Command, `echo "hello\nworld"`)
}

// helpers

func assertEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func assertIntEqual(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func assertAssert(t *testing.T, got, want types.Assert) {
	t.Helper()
	if got.Query != want.Query || got.Predicate != want.Predicate || got.Value != want.Value || got.Negated != want.Negated {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
