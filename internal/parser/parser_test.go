package parser

import (
	"strings"
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
[asserts]
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
[asserts]
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
[asserts]
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
[captures]
pid = stdout
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
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
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
	_, errs := ParseFile(input)
	if len(errs) == 0 {
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

// --- Change 2: section headers must be lowercase ---

func TestParseUppercaseAssertsReturnsError(t *testing.T) {
	input := `echo "hello"
EXIT 0
[Asserts]
stdout contains "hello"
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for uppercase [Asserts], got nil")
	}
	if !strings.Contains(err.Error(), "lowercase") {
		t.Fatalf("expected 'lowercase' in error, got: %v", err)
	}
}

func TestParseUppercaseCapturesReturnsError(t *testing.T) {
	input := `echo "hello"
EXIT 0
[Captures]
val: stdout
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for uppercase [Captures], got nil")
	}
}

func TestParseUppercasePromptsReturnsError(t *testing.T) {
	input := `@timeout 1000
printf "Name: " && read x && echo hi
EXIT 0
[Prompts]
"Name:" => "x"
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for uppercase [Prompts], got nil")
	}
}

// --- Change 1: # lines in implicit body are comments ---

func TestParseBodyCommentIsSkipped(t *testing.T) {
	input := `echo "hello"
EXIT 0
# this is a comment, not expected output
hello
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries[0].Body) != 1 || entries[0].Body[0] != "hello" {
		t.Fatalf("expected body [hello], got %v", entries[0].Body)
	}
}

func TestParseBodyFencedHashIsLiteral(t *testing.T) {
	input := "echo \"#hello\"\nEXIT 0\n```\n# literal hash line\n```\n"
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries[0].Body) != 1 || entries[0].Body[0] != "# literal hash line" {
		t.Fatalf("fenced body: expected [# literal hash line], got %v", entries[0].Body)
	}
}

// --- Change 4: capture syntax uses = instead of : ---

func TestParseCaptureEqualsSign(t *testing.T) {
	input := `echo "42"
EXIT 0
[captures]
result = stdout
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := entries[0].Captures[0]
	assertEqual(t, c.Name, "result")
	assertEqual(t, c.Query, "stdout")
}

func TestParseCaptureOldColonSyntaxReturnsError(t *testing.T) {
	input := `echo "42"
EXIT 0
[captures]
result: stdout
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for old colon capture syntax, got nil")
	}
	if !strings.Contains(err.Error(), "=") {
		t.Fatalf("expected '=' hint in error, got: %v", err)
	}
}

// --- Change 6: bare-value semantics ---

func TestParseBareValueNoQuotes(t *testing.T) {
	input := `echo "hello world"
EXIT 0
[asserts]
stdout contains hello world
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "contains", Value: "hello world",
	})
}

func TestParseBareValueQuotedAndUnquotedEquivalent(t *testing.T) {
	quoted := `echo test
EXIT 0
[asserts]
stdout contains "hello world"
`
	bare := `echo test
EXIT 0
[asserts]
stdout contains hello world
`
	e1, err1 := Parse(quoted)
	e2, err2 := Parse(bare)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v / %v", err1, err2)
	}
	assertAssert(t, e1[0].Asserts[0], e2[0].Asserts[0])
}

func TestParseBareValueNumeric(t *testing.T) {
	input := `echo "42"
EXIT 0
[asserts]
exit == 0
lineCount == 1
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{Query: "exit", Predicate: "==", Value: "0"})
	assertAssert(t, entries[0].Asserts[1], types.Assert{Query: "lineCount", Predicate: "==", Value: "1"})
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
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
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
[captures]
bgpid = pid
[asserts]
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
	if e.Directives.Timeout == nil || *e.Directives.Timeout != 5000 {
		t.Fatalf("expected Timeout 5000, got %v", e.Directives.Timeout)
	}
	if e.Directives.Poll == nil || *e.Directives.Poll != 200 {
		t.Fatalf("expected Poll 200, got %v", e.Directives.Poll)
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
[prompts]
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
[prompts]
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
[prompts]
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
[prompts]
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
[prompts]
"Name:" => "Bob"
[asserts]
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
[asserts]
stderr contains "ready"
later stderr contains "later output"
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
[asserts]
later stdout not contains "error"
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
[asserts]
stderr contains "ready"
[finally]
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
[finally]
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
[finally]
TERM EXIT 0
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for [finally] on non-EXIT NEVER entry")
	}
}

func TestParseFinallyInvalidSignal(t *testing.T) {
	input := `sh -c 'sleep 999'
EXIT NEVER
[finally]
USR1 EXIT 0
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for unsupported signal")
	}
}

func TestParseEnvDirective_EntryLevel(t *testing.T) {
	input := `@env FOO=bar
@env BAZ=qux
echo test
EXIT 0
`
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	if len(f.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f.Entries))
	}
	env := f.Entries[0].Directives.Env
	if len(env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(env))
	}
	assertEqual(t, env["FOO"], "bar")
	assertEqual(t, env["BAZ"], "qux")
}

func TestParseEnvDirective_ValueContainsEquals(t *testing.T) {
	input := `@env CONN=host=localhost;port=5432
echo test
EXIT 0
`
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	assertEqual(t, f.Entries[0].Directives.Env["CONN"], "host=localhost;port=5432")
}

func TestParseEnvDirective_EmptyValue(t *testing.T) {
	input := `@env KEY=
echo test
EXIT 0
`
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	env := f.Entries[0].Directives.Env
	val, exists := env["KEY"]
	if !exists {
		t.Fatal("expected KEY to exist in env")
	}
	if val != "" {
		t.Errorf("expected empty value, got %q", val)
	}
}

func TestParseEnvDirective_NoEquals_Error(t *testing.T) {
	input := `@env NOVALUE
echo test
EXIT 0
`
	_, errs := ParseFile(input)
	if len(errs) == 0 {
		t.Fatal("expected error for @env without =, got none")
	}
	if !strings.Contains(errs[0].Error(), "@env") {
		t.Fatalf("expected @env error, got: %v", errs[0])
	}
}

func TestParseEnvDirective_FileLevel(t *testing.T) {
	input := `---
@env API_URL=http://localhost:8080
@env DEBUG=true
---

echo test
EXIT 0
`
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	env := f.Directives.Env
	if len(env) != 2 {
		t.Fatalf("expected 2 file-level env vars, got %d", len(env))
	}
	assertEqual(t, env["API_URL"], "http://localhost:8080")
	assertEqual(t, env["DEBUG"], "true")
}

func TestParseEnvDirective_DuplicateKey_LastWins(t *testing.T) {
	input := `@env X=first
@env X=second
echo test
EXIT 0
`
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	assertEqual(t, f.Entries[0].Directives.Env["X"], "second")
}

// --- Multi-line assert (triple-quote) tests ---

func TestParseMultilineAssert_Basic(t *testing.T) {
	input := `printf "hello\nworld"
EXIT 0
[asserts]
stdout == """
hello
world
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "==", Value: "hello\nworld",
	})
}

func TestParseMultilineAssert_SingleLineContent(t *testing.T) {
	input := `echo hello
EXIT 0
[asserts]
stdout == """
hello
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "==", Value: "hello",
	})
}

func TestParseMultilineAssert_EmptyContent(t *testing.T) {
	input := `echo -n ""
EXIT 0
[asserts]
stdout == """
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "==", Value: "",
	})
}

func TestParseMultilineAssert_Contains(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout contains """
foo
bar
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "contains", Value: "foo\nbar",
	})
}

func TestParseMultilineAssert_StartsWith(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout startsWith """
foo
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "startsWith", Value: "foo",
	})
}

func TestParseMultilineAssert_EndsWith(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout endsWith """
bar
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "endsWith", Value: "bar",
	})
}

func TestParseMultilineAssert_MatchesRejected(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout matches """
.*
"""
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for matches with triple-quote, got nil")
	}
}

func TestParseMultilineAssert_Negated(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout not contains """
bad
stuff
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "contains", Value: "bad\nstuff", Negated: true,
	})
}

func TestParseMultilineAssert_LaterModifier(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
later stdout contains """
foo
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "contains", Value: "foo", Later: true,
	})
}

func TestParseMultilineAssert_PreservesIndentation(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout == """
  indented
    more
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "==", Value: "  indented\n    more",
	})
}

func TestParseMultilineAssert_BlankLinesInContent(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout == """
foo

bar
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "==", Value: "foo\n\nbar",
	})
}

func TestParseMultilineAssert_Unclosed(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout == """
hello
`
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for unclosed triple-quote, got nil")
	}
}

func TestParseMultilineAssert_EqualsOperator(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout == """
hello
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "==", Value: "hello",
	})
}

func TestParseMultilineAssert_MixedWithSingleLine(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
stdout contains """
hello
world
"""
exit == 0
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries[0].Asserts) != 2 {
		t.Fatalf("expected 2 asserts, got %d", len(entries[0].Asserts))
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "stdout", Predicate: "contains", Value: "hello\nworld",
	})
	assertAssert(t, entries[0].Asserts[1], types.Assert{
		Query: "exit", Predicate: "==", Value: "0",
	})
}

func TestParseMultilineAssert_LineNQuery(t *testing.T) {
	input := `echo test
EXIT 0
[asserts]
line 1 == """
foo
"""
`
	entries, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAssert(t, entries[0].Asserts[0], types.Assert{
		Query: "line 1", Predicate: "==", Value: "foo",
	})
}
