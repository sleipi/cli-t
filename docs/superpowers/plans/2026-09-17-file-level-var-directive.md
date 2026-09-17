# File-level `@var` directive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a frontmatter directive `@var KEY=VALUE` that sets a file-scoped default for the `{{...}}` template system, so sibling `.clitest` files that differ by one value need only change one frontmatter line instead of repeating the value on every line that uses it.

**Architecture:** `@var` is parsed exactly like the existing `@env` directive (file-level only, into a new `FileDirectives.Var map[string]string`), with a duplicate-key check `@env` doesn't have. Resolution needs no change to the existing load-time `{{...}}` substitution pass (`--var` + `$ENV`, unchanged). Instead, the single `captures` map created once per file run and threaded by reference through every executor call is seeded with the file's `@var` values at creation. Real captures then overwrite seeded values on the same map via ordinary map-write semantics — giving `--var > [captures] > @var` for free, with zero changes inside the executor package.

**Tech Stack:** Go (stdlib only — `strings`, `strconv`), existing `.clitest`-file-based test suite (unit tests via `go test`, e2e via the `clitest` binary running `.clitest` fixture files against itself).

**Spec:** `docs/superpowers/specs/2026-09-17-file-level-var-directive-design.md`

## Global Constraints

- File-level only — no `EntryDirectives.Var`. An `@var` line placed after the frontmatter is silently ignored, same as any other unknown directive (`SPEC.md:636`, unchanged).
- Duplicate `@var` key within one frontmatter block is a hard parse error (`*parser.DirectiveError`), not last-write-wins.
- Malformed `@var` (no `=`) is a hard parse error, same message shape as `@env`'s.
- `@var` never touches `@env`/`$VAR` — those remain a fully separate mechanism.

---

### Task 1: Parse `@var` directive (file-level, with duplicate detection)

**Files:**
- Modify: `internal/types/types.go` (`FileDirectives` struct, ~line 18-25)
- Modify: `internal/parser/directives.go` (`interpretFileDirectives`, ~line 93-126)
- Test: `internal/parser/parser_test.go`

**Interfaces:**
- Produces: `types.FileDirectives.Var map[string]string` — nil if no `@var` in the file, otherwise `KEY -> VALUE`. Consumed by Task 3 (`cmd/clitest/root.go`, reading `parsed.Directives.Var`).

- [ ] **Step 1: Add the `Var` field to `FileDirectives`**

In `internal/types/types.go`, add to the `FileDirectives` struct (next to the existing `Env` field):

```go
// FileDirectives holds interpreted directives for a file.
type FileDirectives struct {
	Groups     []string
	Skip       bool
	SkipReason string
	Timeout    *int              // @timeout in ms (nil = not set)
	Workdir    string            // @workdir path (empty = not set)
	Env        map[string]string // @env KEY=VALUE (nil = not set)
	Var        map[string]string // @var KEY=VALUE (nil = not set, file-level only)
}
```

- [ ] **Step 2: Write the failing tests**

Add to `internal/parser/parser_test.go` (near the existing `TestParseEnvDirective_*` tests, ~line 850):

```go
func TestParseVarDirective_FileLevel(t *testing.T) {
	input := `---
@var SCENARIO=checkout/case-a
@var PHP=php81
---

echo test
EXIT 0
`
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	v := f.Directives.Var
	if len(v) != 2 {
		t.Fatalf("expected 2 file-level vars, got %d", len(v))
	}
	assertEqual(t, v["SCENARIO"], "checkout/case-a")
	assertEqual(t, v["PHP"], "php81")
}

func TestParseVarDirective_ValueContainsEquals(t *testing.T) {
	input := `---
@var CONN=host=localhost;port=5432
---

echo test
EXIT 0
`
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	assertEqual(t, f.Directives.Var["CONN"], "host=localhost;port=5432")
}

func TestParseVarDirective_EmptyValue(t *testing.T) {
	input := `---
@var KEY=
---

echo test
EXIT 0
`
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	val, exists := f.Directives.Var["KEY"]
	if !exists {
		t.Fatal("expected KEY to exist in var")
	}
	if val != "" {
		t.Errorf("expected empty value, got %q", val)
	}
}

func TestParseVarDirective_NoEquals_Error(t *testing.T) {
	input := `---
@var NOVALUE
---

echo test
EXIT 0
`
	_, errs := ParseFile(input)
	if len(errs) == 0 {
		t.Fatal("expected error for @var without =, got none")
	}
	if !strings.Contains(errs[0].Error(), "@var") {
		t.Fatalf("expected @var error, got: %v", errs[0])
	}
}

func TestParseVarDirective_DuplicateKey_Error(t *testing.T) {
	input := `---
@var X=first
@var X=second
---

echo test
EXIT 0
`
	_, errs := ParseFile(input)
	if len(errs) == 0 {
		t.Fatal("expected error for duplicate @var key, got none")
	}
	if !strings.Contains(errs[0].Error(), "@var") || !strings.Contains(errs[0].Error(), "X") {
		t.Fatalf("expected @var duplicate-key error mentioning X, got: %v", errs[0])
	}
}

func TestParseVarDirective_EntryLevelIgnored(t *testing.T) {
	// @var after the frontmatter is an unknown entry directive — silently
	// ignored, same as any other unrecognized @directive (SPEC.md).
	input := `@var NAME=foo
echo {{NAME}}
EXIT 0
`
	f, errs := ParseFile(input)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	if f.Entries[0].Command != "echo {{NAME}}" {
		t.Errorf("expected placeholder left literal, got %q", f.Entries[0].Command)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/parser/... -run TestParseVarDirective -v`
Expected: FAIL — `f.Directives.Var` doesn't compile yet (undefined field).

- [ ] **Step 4: Implement the `var` case in `interpretFileDirectives`**

In `internal/parser/directives.go`, add a `case "var":` inside the `switch d.Name` in `interpretFileDirectives` (after the existing `case "env":` block, ~line 122). Match the existing `if`/`else` style used by the `env`/`timeout` cases right above it (reading `f.Directives.Var[parts[0]]` on a possibly-nil map is safe in Go — it returns `"", false`):

```go
		case "var":
			parts := strings.SplitN(d.Value, "=", 2)
			if len(parts) < 2 || parts[0] == "" {
				errs = append(errs, &DirectiveError{Line: d.Line, Directive: d.Name, Message: fmt.Sprintf("value must be KEY=VALUE with non-empty key, got %q", d.Value)})
			} else if _, exists := f.Directives.Var[parts[0]]; exists {
				errs = append(errs, &DirectiveError{Line: d.Line, Directive: d.Name, Message: fmt.Sprintf("duplicate key %q (already defined earlier in this file)", parts[0])})
			} else {
				if f.Directives.Var == nil {
					f.Directives.Var = make(map[string]string)
				}
				f.Directives.Var[parts[0]] = parts[1]
			}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/parser/... -run TestParseVarDirective -v`
Expected: PASS (all 6 subtests)

- [ ] **Step 6: Run the full parser package test suite to check for regressions**

Run: `go test ./internal/parser/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/types/types.go internal/parser/directives.go internal/parser/parser_test.go
git commit -m "feat: parse file-level @var directive"
```

---

### Task 2: `vars.SeedCaptures` helper

**Files:**
- Modify: `internal/vars/vars.go`
- Test: `internal/vars/vars_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `vars.SeedCaptures(fileVars map[string]string) map[string]string` — returns a new map containing a copy of `fileVars` (empty, non-nil map if `fileVars` is nil or empty). Consumed by Task 3.

- [ ] **Step 1: Write the failing tests**

Add to `internal/vars/vars_test.go`:

```go
func TestSeedCaptures_CopiesValues(t *testing.T) {
	fileVars := map[string]string{"NAME": "world"}
	got := SeedCaptures(fileVars)
	if got["NAME"] != "world" {
		t.Errorf("expected 'world', got %q", got["NAME"])
	}
}

func TestSeedCaptures_DoesNotAliasInput(t *testing.T) {
	fileVars := map[string]string{"NAME": "world"}
	got := SeedCaptures(fileVars)
	got["NAME"] = "mutated"
	if fileVars["NAME"] != "world" {
		t.Errorf("expected original map untouched, got %q", fileVars["NAME"])
	}
}

func TestSeedCaptures_NilInput(t *testing.T) {
	got := SeedCaptures(nil)
	if got == nil {
		t.Fatal("expected non-nil map for nil input")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/vars/... -run TestSeedCaptures -v`
Expected: FAIL — `SeedCaptures` undefined.

- [ ] **Step 3: Implement `SeedCaptures`**

Add to `internal/vars/vars.go` (after `SubstituteCaptures`):

```go
// SeedCaptures returns a new map pre-populated with fileVars, for use as the
// starting point of an entry's captures map. The returned map never aliases
// fileVars, so later writes (real captures) never mutate the caller's map.
func SeedCaptures(fileVars map[string]string) map[string]string {
	seeded := make(map[string]string, len(fileVars))
	for k, v := range fileVars {
		seeded[k] = v
	}
	return seeded
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/vars/... -run TestSeedCaptures -v`
Expected: PASS (all 3 subtests)

- [ ] **Step 5: Run the full vars package test suite to check for regressions**

Run: `go test ./internal/vars/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/vars/vars.go internal/vars/vars_test.go
git commit -m "feat: add vars.SeedCaptures helper"
```

---

### Task 3: Wire `@var` into the run — seed the captures map from `parsed.Directives.Var`

**Files:**
- Modify: `cmd/clitest/run.go` (`runEntries`, `runEntriesVerbose`, `runEntriesCompact`)
- Modify: `cmd/clitest/root.go` (`processFile`, both call sites)
- Modify: `cmd/clitest/run_test.go` (existing `runEntries` call sites)
- Test: `cmd/clitest/run_test.go` (new end-to-end test)

**Interfaces:**
- Consumes: `types.FileDirectives.Var` (Task 1), `vars.SeedCaptures` (Task 2).
- Produces: `runEntries(cfg *runConfig, pd *display.ProgressDisplay, fileIdx int, entries []types.Entry, fileVars map[string]string, onResult func(entryOutcome)) (pass, fail, skip int, details []display.CompactFailure)` — new `fileVars` parameter inserted before `onResult`. Same signature-shape addition on `runEntriesVerbose` and `runEntriesCompact` (see steps below for exact placement).

- [ ] **Step 1: Write the failing end-to-end test**

Add to `cmd/clitest/run_test.go`:

```go
func TestRunEntries_FileVarResolvesInCommand(t *testing.T) {
	entries := []types.Entry{
		{Command: "echo {{NAME}}", Asserts: []types.Assert{{Query: "stdout", Predicate: "contains", Value: "world"}}},
	}
	fileVars := map[string]string{"NAME": "world"}

	cancelled := &atomic.Bool{}
	cfg := &runConfig{FailFast: false, Cancelled: cancelled}
	pd := newTestProgressDisplay()

	pass, fail, _, _ := runEntries(cfg, pd, 0, entries, fileVars, nil)

	if fail != 0 {
		t.Errorf("expected 0 failures, got %d", fail)
	}
	if pass != 1 {
		t.Errorf("expected 1 pass, got %d", pass)
	}
}

func TestRunEntries_CaptureOverridesFileVar(t *testing.T) {
	entries := []types.Entry{
		{
			Command:  "echo captured_val",
			Captures: []types.Capture{{Name: "NAME", Source: types.CaptureStdout}},
			Asserts:  []types.Assert{{Query: "exit", Predicate: "eq", Value: "0"}},
		},
		{
			Command: "echo {{NAME}}",
			Asserts: []types.Assert{{Query: "stdout", Predicate: "contains", Value: "captured_val"}},
		},
	}
	fileVars := map[string]string{"NAME": "default_val"}

	cancelled := &atomic.Bool{}
	cfg := &runConfig{FailFast: false, Cancelled: cancelled}
	pd := newTestProgressDisplay()

	pass, fail, _, _ := runEntries(cfg, pd, 0, entries, fileVars, nil)

	if fail != 0 {
		t.Errorf("expected 0 failures, got %d", fail)
	}
	if pass != 2 {
		t.Errorf("expected 2 pass, got %d", pass)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/clitest/... -run TestRunEntries_FileVarResolvesInCommand -v`
Expected: FAIL — compile error, `runEntries` called with wrong number of arguments.

- [ ] **Step 3: Update `runEntries` signature and seed `captures`**

In `cmd/clitest/run.go`, change the `runEntries` signature (~line 44) and the `captures` initialization (~line 47):

```go
func runEntries(cfg *runConfig, pd *display.ProgressDisplay, fileIdx int, entries []types.Entry, fileVars map[string]string, onResult func(entryOutcome)) (pass, fail, skip int, details []display.CompactFailure) {
	regular, defers := executor.SplitDeferEntries(entries)
	pd.UpdateProgress(fileIdx, 0, len(regular))
	captures := vars.SeedCaptures(fileVars)
	var backgrounds []*executor.BackgroundResult
```

- [ ] **Step 4: Thread `fileVars` through `runEntriesVerbose` and `runEntriesCompact`**

In `cmd/clitest/run.go`, update `runEntriesVerbose` (~line 149) — add the parameter and pass it through to `runEntries` (~line 168):

```go
func runEntriesVerbose(cfg *runConfig, vd *display.VerboseDisplay, pd *display.ProgressDisplay, fileIdx int, entries []types.Entry, fileVars map[string]string) (pass, fail, skip int) {
```

```go
	pass, fail, skip, _ = runEntries(cfg, pd, fileIdx, entries, fileVars, onResult)
	return
```

And `runEntriesCompact` (~line 173):

```go
func runEntriesCompact(cfg *runConfig, pd *display.ProgressDisplay, fileIdx int, entries []types.Entry, fileVars map[string]string) (pass, fail, skip int, details []display.CompactFailure) {
	return runEntries(cfg, pd, fileIdx, entries, fileVars, nil)
}
```

- [ ] **Step 5: Update the two call sites in `cmd/clitest/root.go`**

In `processFile` (~line 234 and ~line 240):

```go
		pass, fail, skip := runEntriesVerbose(cfg, vd, pd, idx, entries, parsed.Directives.Var)
```

```go
	pass, fail, skip, details := runEntriesCompact(cfg, pd, idx, entries, parsed.Directives.Var)
```

- [ ] **Step 6: Update the existing `runEntries` call sites in `cmd/clitest/run_test.go`**

`TestRunEntries_FailFastCancels` (~line 70) and `TestRunEntries_PreCancelledSkipsAll` (~line 97) currently call `runEntries(cfg, pd, 0, entries, nil)` — insert `nil` for `fileVars` before the trailing `onResult` argument:

```go
	pass, fail, skip, _ := runEntries(cfg, pd, 0, entries, nil, nil)
```

(apply to both test functions)

- [ ] **Step 7: Run the new tests to verify they pass**

Run: `go test ./cmd/clitest/... -run TestRunEntries -v`
Expected: PASS (including the two pre-existing `TestRunEntries_*` tests and the two new ones)

- [ ] **Step 8: Run the full repo test suite to check for regressions**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add cmd/clitest/run.go cmd/clitest/root.go cmd/clitest/run_test.go
git commit -m "feat: seed captures map from file-level @var, wire through run path"
```

---

### Task 4: Document `@var` in `SPEC.md` and add `examples/09_var.clitest`

**Files:**
- Modify: `SPEC.md`
- Create: `examples/09_var.clitest`

**Interfaces:**
- Consumes: the working `@var` feature from Tasks 1-3 (this task's example file is executed by `make examples`, which is a form of test).
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Update the Variables priority list**

In `SPEC.md`, replace the priority list at line 432-436:

```markdown
Variables are resolved from (in priority order):
1. `--var NAME=VALUE` CLI flags
2. `[captures]` from previous entries
3. `@var NAME=VALUE` file-level frontmatter directive
4. Environment variables (via `$VAR` or `${VAR}` syntax)
```

- [ ] **Step 2: Add `@var` to the Available Directives table**

In `SPEC.md`, add a row after the `@env` row (line 499):

```markdown
| `@var`    | file        | `@var KEY=VALUE` | Set a default for `{{KEY}}` template substitution, scoped to this file |
```

- [ ] **Step 3: Add the `#### @var` subsection**

In `SPEC.md`, insert after the `#### @env` subsection (after line 622, before `#### Directive Validation`):

```markdown
#### `@var`

Sets a file-scoped default value for `{{KEY}}` template substitution. File-level only (frontmatter) — placing `@var` after the frontmatter has no effect (unknown entry directives are silently ignored). One `KEY=VALUE` per directive. A duplicate key within the same file's frontmatter is a parse error.

**Precedence:** `--var` (CLI) > `[captures]` > `@var` (file) — see [Variables](#variables).

**Values:** The value is everything after the first `=`. Values may contain `=`, spaces, and special characters. Empty values (`@var KEY=`) are valid. Malformed directives without `=` are rejected at parse time, as is a repeated key.

**Use case:** sibling `.clitest` files that differ by exactly one value (a scenario directory, a tool version) can express that value once, instead of repeating it on every line that needs it:

```
---
@var SCENARIO=checkout/case-a
---

hurl --test tests/{{SCENARIO}}/stubs/_register.hurl
EXIT 0

php bin/console app:seed tests/{{SCENARIO}}/seller.json
EXIT 0
```

A sibling file for `case-b` differs from this one only in the frontmatter line.
```

- [ ] **Step 4: Add `@var` to the Directive Validation table**

In `SPEC.md`, add a row to the validation table (after line 634):

```markdown
| `@var` | `KEY=VALUE` with non-empty key | Empty value after `=` is valid. Missing `=` is rejected. Duplicate key within one file is rejected. |
```

- [ ] **Step 5: Write `examples/09_var.clitest`**

```
---
@var SCENARIO=checkout/case-a
---

# All entries in this file see {{SCENARIO}}
echo "running {{SCENARIO}}"
EXIT 0
running checkout/case-a

# A sibling file for case-b would differ only in the frontmatter line above
echo "path: tests/{{SCENARIO}}/seller.json"
EXIT 0
path: tests/checkout/case-a/seller.json
```

- [ ] **Step 6: Run the examples suite to verify it passes**

Run: `make build examples`
Expected: all examples pass, including `09_var.clitest`

- [ ] **Step 7: Commit**

```bash
git add SPEC.md examples/09_var.clitest
git commit -m "docs: document @var directive, add example"
```

---

### Task 5: e2e positive-behavior tests for `@var`

**Files:**
- Create: `test/e2e/var/it_sets_file_level_var.clitest`
- Create: `test/e2e/var/it_var_available_to_all_entries.clitest`
- Create: `test/e2e/var/it_captures_override_var.clitest`
- Create: `test/e2e/var/it_cli_var_overrides_var.clitest`

**Interfaces:**
- Consumes: the working `@var` feature from Tasks 1-3.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: `it_sets_file_level_var.clitest`**

```
---
@var NAME=world
---

# File-level var resolves in the command
echo "hello {{NAME}}"
hello world
```

- [ ] **Step 2: `it_var_available_to_all_entries.clitest`**

```
---
@var SCENARIO=case-a
---

# First entry sees the file-level var
echo "{{SCENARIO}}"
case-a

# Second entry also sees it
echo "path/{{SCENARIO}}/file"
path/case-a/file
```

- [ ] **Step 3: `it_captures_override_var.clitest`**

```
---
@var NAME=default_val
---

# Capture a value under the same key as the file-level var
echo "captured_val"
EXIT 0
[captures]
NAME stdout

# Subsequent entries see the captured value, not the file-level default
echo "{{NAME}}"
captured_val
```

- [ ] **Step 4: `it_cli_var_overrides_var.clitest`**

```
---
@var NAME=file_value
---

# --var (passed on the command line invoking this fixture) beats @var
echo "{{NAME}}"
cli_value
```

This fixture is invoked with `--var NAME=cli_value` by a wrapper entry in `test/e2e/syntax/` — **do not** add it directly to `test/e2e/var/` for standalone execution, since `make e2e` runs `./clitest --silent test/e2e/` with no `--var` flag and this fixture would then see `file_value`, failing the assertion. Instead:

- [ ] **Step 4a: Move `it_cli_var_overrides_var.clitest` under `test/_fixtures/syntax/`**

Move the file to `test/_fixtures/syntax/var_cli_override.clitest` (same content as Step 4).

- [ ] **Step 4b: Add the wrapper entry**

Append to `test/e2e/syntax/it_validates_directives.clitest`:

```

# Validates that --var overrides file-level @var
./clitest --var NAME=cli_value test/_fixtures/syntax/var_cli_override.clitest
EXIT 0
```

- [ ] **Step 5: Run the e2e suite to verify all new fixtures pass**

Run: `make build e2e`
Expected: all e2e tests pass, including the 3 files under `test/e2e/var/` and the new wrapper entry in `test/e2e/syntax/it_validates_directives.clitest`

- [ ] **Step 6: Commit**

```bash
git add test/e2e/var/ test/_fixtures/syntax/var_cli_override.clitest test/e2e/syntax/it_validates_directives.clitest
git commit -m "test: add e2e coverage for @var resolution and precedence"
```

---

### Task 6: e2e negative-case tests — malformed and duplicate `@var`

**Files:**
- Create: `test/_fixtures/syntax/bad_var.clitest`
- Create: `test/_fixtures/syntax/duplicate_var.clitest`
- Modify: `test/e2e/syntax/it_validates_directives.clitest`

**Interfaces:**
- Consumes: the parse-time validation from Task 1.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: `test/_fixtures/syntax/bad_var.clitest`**

```
---
@var NOEQUALS
---

echo hello
EXIT 0
```

- [ ] **Step 2: `test/_fixtures/syntax/duplicate_var.clitest`**

```
---
@var X=first
@var X=second
---

echo hello
EXIT 0
```

- [ ] **Step 3: Append validation entries to `test/e2e/syntax/it_validates_directives.clitest`**

```

# Validates that malformed @var is rejected at parse time
./clitest test/_fixtures/syntax/bad_var.clitest
EXIT 1
[asserts]
stdout contains "@var"
stdout contains "KEY=VALUE"

# Validates that duplicate @var key is rejected at parse time
./clitest test/_fixtures/syntax/duplicate_var.clitest
EXIT 1
[asserts]
stdout contains "@var"
stdout contains "duplicate"
```

- [ ] **Step 4: Run the e2e suite to verify the new negative cases pass**

Run: `make build e2e`
Expected: all e2e tests pass, including the 2 new validation entries

- [ ] **Step 5: Run the full repo test suite one final time**

Run: `make test e2e examples`
Expected: everything passes — unit tests, e2e tests, examples

- [ ] **Step 6: Commit**

```bash
git add test/_fixtures/syntax/bad_var.clitest test/_fixtures/syntax/duplicate_var.clitest test/e2e/syntax/it_validates_directives.clitest
git commit -m "test: add e2e coverage for malformed and duplicate @var"
```
