package parser

import (
	"testing"

	"github.com/sleipi/cli-t/internal/types"
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

func TestParseEntryGroupDirective(t *testing.T) {
	input := `# Test with groups
@group BUG-1234 smoke
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
	assertEqual(t, e.Comment, "# Test with groups")
	if len(e.Directives.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d: %v", len(e.Directives.Groups), e.Directives.Groups)
	}
	assertEqual(t, e.Directives.Groups[0], "BUG-1234")
	assertEqual(t, e.Directives.Groups[1], "smoke")
}

func TestParseEntrySkipDirective(t *testing.T) {
	input := `# Broken test
@skip known flaky on CI
echo "flaky"
EXIT 0
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := entries[0]
	if !e.Directives.Skip {
		t.Fatal("expected entry to be skipped")
	}
	assertEqual(t, e.Directives.SkipReason, "known flaky on CI")
}

func TestParseEntrySkipBare(t *testing.T) {
	input := `@skip
echo "skip me"
EXIT 0
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !entries[0].Directives.Skip {
		t.Fatal("expected entry to be skipped")
	}
	assertEqual(t, entries[0].Directives.SkipReason, "")
}

func TestParseFrontmatter(t *testing.T) {
	input := `---
@group BUG-1234 performance
@skip waiting for backend fix
---

echo "hello"
EXIT 0
`
	f, err := ParseFile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Directives.Groups) != 2 {
		t.Fatalf("expected 2 file groups, got %d: %v", len(f.Directives.Groups), f.Directives.Groups)
	}
	assertEqual(t, f.Directives.Groups[0], "BUG-1234")
	assertEqual(t, f.Directives.Groups[1], "performance")
	if !f.Directives.Skip {
		t.Fatal("expected file to be skipped")
	}
	assertEqual(t, f.Directives.SkipReason, "waiting for backend fix")
	if len(f.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f.Entries))
	}
}

func TestParseFrontmatterUnclosed(t *testing.T) {
	input := `---
@group test
echo "hello"
`
	_, err := ParseFile(input)
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter")
	}
}

func TestParseDirectiveAfterCommandErrors(t *testing.T) {
	input := `echo "hello"
@group smoke
EXIT 0
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for directive after command")
	}
}

func TestParseMultipleDirectives(t *testing.T) {
	input := `# Test
@group smoke
@skip WIP
echo "test"
EXIT 0
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := entries[0]
	if len(e.Directives.Groups) != 1 || e.Directives.Groups[0] != "smoke" {
		t.Fatalf("expected groups [smoke], got %v", e.Directives.Groups)
	}
	if !e.Directives.Skip {
		t.Fatal("expected skip")
	}
	assertEqual(t, e.Directives.SkipReason, "WIP")
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

func TestParseFrontmatterWithProseText(t *testing.T) {
	input := `---
This is a description of the test file
@group examples basics

Add your tests below
---

echo "hello"
`
	f, err := ParseFile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Directives.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(f.Directives.Groups))
	}
	assertEqual(t, f.Directives.Groups[0], "examples")
	assertEqual(t, f.Directives.Groups[1], "basics")
	if len(f.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f.Entries))
	}
}

func assertAssert(t *testing.T, got, want types.Assert) {
	t.Helper()
	if got.Query != want.Query || got.Predicate != want.Predicate || got.Value != want.Value || got.Negated != want.Negated || got.Later != want.Later {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseExitNever(t *testing.T) {
	input := `@timeout 5000
@poll 200
sh -c 'echo "ready"; sleep 999'
EXIT NEVER
[Captures]
bgpid: pid
[Asserts]
stdout contains "ready"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if !e.ExitNever {
		t.Fatal("expected ExitNever to be true")
	}
	if e.Directives.Timeout != 5000 {
		t.Fatalf("expected Timeout 5000, got %d", e.Directives.Timeout)
	}
	if e.Directives.Poll != 200 {
		t.Fatalf("expected Poll 200, got %d", e.Directives.Poll)
	}
	if len(e.Captures) != 1 || e.Captures[0].Query != "pid" {
		t.Fatalf("expected pid capture, got %+v", e.Captures)
	}
	if len(e.Asserts) != 1 || e.Asserts[0].Query != "stdout" {
		t.Fatalf("expected stdout assert, got %+v", e.Asserts)
	}
}

func TestParseDefer(t *testing.T) {
	input := `@defer
kill 12345
EXIT 0

@defer
rm /tmp/testfile
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if !entries[0].Directives.Defer {
		t.Fatal("expected first entry to be defer")
	}
	assertEqual(t, entries[0].Command, "kill 12345")
	if !entries[1].Directives.Defer {
		t.Fatal("expected second entry to be defer")
	}
	assertEqual(t, entries[1].Command, "rm /tmp/testfile")
}

func TestParsePromptsSubstring(t *testing.T) {
	input := `@timeout 5000
printf "Enter name: " && read name && echo "Hello $name"
EXIT 0
[Prompts]
"Enter name:" => "Alice"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if len(e.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(e.Prompts))
	}
	p := e.Prompts[0]
	assertEqual(t, p.Pattern, "Enter name:")
	assertEqual(t, p.Response, "Alice")
	if p.IsRegex {
		t.Fatal("expected IsRegex to be false")
	}
	assertIntEqual(t, p.Repeat, 1)
}

func TestParsePromptsRegex(t *testing.T) {
	input := `@timeout 3000
./installer.sh
EXIT 0
[Prompts]
/Continue\? \[y\/n\]/ => "yes"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := entries[0].Prompts[0]
	assertEqual(t, p.Pattern, `Continue\? \[y\/n\]`)
	assertEqual(t, p.Response, "yes")
	if !p.IsRegex {
		t.Fatal("expected IsRegex to be true")
	}
}

func TestParsePromptsMultiplier(t *testing.T) {
	input := `@timeout 3000
./setup.sh
EXIT 0
[Prompts]
"Continue?" => "yes" * 3
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := entries[0].Prompts[0]
	assertEqual(t, p.Pattern, "Continue?")
	assertEqual(t, p.Response, "yes")
	assertIntEqual(t, p.Repeat, 3)
}

func TestParsePromptsMultipleEntries(t *testing.T) {
	input := `@timeout 5000
php bin/console app:create-user
EXIT 0
[Prompts]
"Enter username:" => "alice"
"Enter email:" => "alice@example.com"
/Confirm .* \[yes\]/ => "yes"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompts := entries[0].Prompts
	if len(prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(prompts))
	}
	assertEqual(t, prompts[0].Pattern, "Enter username:")
	assertEqual(t, prompts[1].Pattern, "Enter email:")
	assertEqual(t, prompts[2].Pattern, `Confirm .* \[yes\]`)
	if !prompts[2].IsRegex {
		t.Fatal("expected third prompt to be regex")
	}
}

func TestParsePromptsWithAsserts(t *testing.T) {
	input := `@timeout 5000
printf "Name: " && read name && echo "Hi $name"
EXIT 0
[Prompts]
"Name:" => "Bob"
[Asserts]
stdout contains "Hi Bob"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := entries[0]
	if len(e.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(e.Prompts))
	}
	if len(e.Asserts) != 1 {
		t.Fatalf("expected 1 assert, got %d", len(e.Asserts))
	}
}

func TestParseLaterModifier(t *testing.T) {
	input := `@timeout 5000
sh -c 'echo "ready"; sleep 999'
EXIT NEVER
[Asserts]
stderr contains "ready"
stderr contains later "later output"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := entries[0]
	if len(e.Asserts) != 2 {
		t.Fatalf("expected 2 asserts, got %d", len(e.Asserts))
	}
	assertAssert(t, e.Asserts[0], types.Assert{Query: "stderr", Predicate: "contains", Value: "ready"})
	assertAssert(t, e.Asserts[1], types.Assert{Query: "stderr", Predicate: "contains", Value: "later output", Later: true})
}

func TestParseLaterWithNegation(t *testing.T) {
	input := `sh -c 'sleep 999'
EXIT NEVER
[Asserts]
stdout not contains later "error"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := entries[0].Asserts[0]
	assertAssert(t, a, types.Assert{Query: "stdout", Predicate: "contains", Value: "error", Negated: true, Later: true})
}

func TestParseFinallySection(t *testing.T) {
	input := `@timeout 5000
sh -c 'echo "ready"; sleep 999'
EXIT NEVER
[Asserts]
stderr contains "ready"
[Finally]
TERM EXIT 0 timeout 3000
stderr contains "shutdown"
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := entries[0]
	if e.Finally == nil {
		t.Fatal("expected Finally section")
	}
	if e.Finally.Signal != "TERM" {
		t.Fatalf("expected signal TERM, got %s", e.Finally.Signal)
	}
	if e.Finally.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", e.Finally.ExitCode)
	}
	if e.Finally.Timeout != 3000 {
		t.Fatalf("expected timeout 3000, got %d", e.Finally.Timeout)
	}
	if len(e.Finally.Asserts) != 1 {
		t.Fatalf("expected 1 finally assert, got %d", len(e.Finally.Asserts))
	}
	assertAssert(t, e.Finally.Asserts[0], types.Assert{Query: "stderr", Predicate: "contains", Value: "shutdown"})
}

func TestParseFinallyDefaultTimeout(t *testing.T) {
	input := `sh -c 'sleep 999'
EXIT NEVER
[Finally]
KILL EXIT 137
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Finally.Timeout != 1000 {
		t.Fatalf("expected default timeout 1000, got %d", entries[0].Finally.Timeout)
	}
}

func TestParseFinallyOnNonExitNeverFails(t *testing.T) {
	input := `echo hello
EXIT 0
[Finally]
TERM EXIT 0
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for [Finally] on non-EXIT NEVER entry")
	}
}

func TestParseFinallyInvalidSignal(t *testing.T) {
	input := `sh -c 'sleep 999'
EXIT NEVER
[Finally]
USR1 EXIT 0
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for unsupported signal")
	}
}
