# AGENTS.md

The key words "MUST", "MUST NOT", "SHOULD", "SHOULD NOT", and "MAY" in this document are to be interpreted as described in [RFC 2119](https://datatracker.ietf.org/doc/html/rfc2119).

## Project

`clitest` (CLI Test) — a Go CLI tool that runs declarative `.clitest` test files against shell commands. Single binary, zero dependencies.

- Module: `github.com/sleipi/cli-t`
- Binary output: `./clitest` (repo root)
- Spec: `SPEC.md` — the authoritative syntax reference; keep it in sync with code changes.

## Tech Stack

* Go

## Workflow

* You MUST work on a feature branch — never commit directly to main.
* You MUST follow the [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) specification for all commit messages.
* You MUST write code using a TDD approach — write/update tests before implementation.
* You MUST write E2E tests (in `test/e2e/`) for any new or changed behaviour.

## Branch Naming

Branch names MUST follow the format `<type>/<short-description>`.

**Types:** `feat/`, `fix/`, `docs/`, `refactor/`, `test/`, `chore/`

**Rules:**
* You MUST use lowercase and hyphens as word separators (e.g. `feat/add-capture-support`).
* You SHOULD keep names short but descriptive.
* You MAY include an issue number (e.g. `feat/42-capture-support`).
* You MUST NOT use special characters beyond hyphens and slashes.

## Commands

```bash
make build        # go build -o clitest ./cmd/clitest/
make test         # go test ./...
make e2e          # build + run ./clitest test/e2e/
make examples     # build + run ./clitest examples/
make all          # test + e2e + examples (use this to verify changes)
```

Always run `make all` after changes — unit tests alone won't catch parser/CLI regressions.

## Package Layout

```
cmd/clitest/main.go            CLI entrypoint, flag parsing, file discovery, parallel execution
cmd/clitest/main_test.go       Unit tests for resolveFiles, glob support
cmd/clitest/display.go         Display interface, ProgressDisplay (compact progress bars, TTY-aware)
cmd/clitest/display_verbose.go VerboseDisplay (per-entry output with checkmarks)
cmd/clitest/display_test.go    Unit tests for both display implementations
internal/parser/            .clitest format parser (entry builder, assert/capture parsing)
internal/runner/            Command execution via sh -c, captures stdout/stderr/exit/duration
internal/assert/            Predicate evaluation engine (queries + predicates + negation)
pkg/types/                  Shared types: Entry, Assert, Capture, File
examples/*.clitest             User-facing usage examples (also validated via `make examples`)
test/e2e/syntax/            E2E tests for .clitest syntax (asserts, captures)
test/e2e/output/            E2E tests for header/footer/summary output and output v2
test/e2e/options/           E2E tests for CLI flags (--var, --parallel, -v)
test/e2e/resolve/           E2E tests for file discovery (recursive, glob, skip warnings)
test/_fixtures/             Shared test fixtures (e.g. intentionally failing .clitest files)
```

## Conventions

- TDD: write/update `*_test.go` before or alongside implementation.
- `test/e2e/` contains all E2E tests of the clitest tool itself — keep them green via `make e2e`.
- `examples/` contains user-facing usage demonstrations — validated via `make examples`.
- Parser is hand-rolled (no parser generator). Entries separated by blank lines; sections by `[Name]` headers.
- Values in asserts: `"quoted"` strings, `/regex/` literals, or bare tokens. See `unquoteValue` in parser.
- Runner always uses `sh -c`; commands are single-line only (multi-line is planned but not implemented).
- Variable substitution (`{{name}}`) happens in `cmd/clitest/main.go` before parsing — it's a simple string replace, not part of the parser.
- File discovery is recursive by default (`--no-recursive` to disable). Non-`.clitest` files are skipped with a warning to stderr.
- Glob patterns in arguments (quoted, e.g. `"examples/*.clitest"`) are expanded by clitest itself, not the shell. The original pattern is preserved in the header output.
- `EXIT NEVER` marks background processes — runner starts them without waiting for exit, polls asserts until pass or `@timeout`.
- `@defer` entries are cleanup (not tests): collected during parsing, executed LIFO at file end, errors logged but not failed.
- `@poll MS` sets the polling interval for `EXIT NEVER` asserts (default 100ms).
- `pid` is a capture query only available on `EXIT NEVER` entries.

# Coding Guidelines

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.