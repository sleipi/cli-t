# AGENTS.md

## Project

`clit` (CLI Test) — a Go CLI tool that runs declarative `.clit` test files against shell commands. Single binary, zero dependencies.

- Module: `github.com/ronny/clit`
- Binary output: `./clit` (repo root)
- Spec: `SPEC.md` — the authoritative syntax reference; keep it in sync with code changes.

## Tech Stack

* Go

## Workflow

* Always work on a feature branch — never commit directly to main.
* Follow the [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) specification for all commit messages.
* Write code using a TDD approach — write/update tests before implementation.
* Always write E2E tests (in `test/e2e/`) for any new or changed behaviour.

## Commands

```bash
make build        # go build -o clit ./cmd/clit/
make test         # go test ./...
make e2e          # build + run ./clit test/e2e/
make examples     # build + run ./clit examples/
make all          # test + e2e + examples (use this to verify changes)
```

Always run `make all` after changes — unit tests alone won't catch parser/CLI regressions.

## Package Layout

```
cmd/clit/main.go            CLI entrypoint, flag parsing, file discovery, parallel execution
cmd/clit/main_test.go       Unit tests for resolveFiles, glob support
cmd/clit/display.go         Display interface, ProgressDisplay (compact progress bars, TTY-aware)
cmd/clit/display_verbose.go VerboseDisplay (per-entry output with checkmarks)
cmd/clit/display_test.go    Unit tests for both display implementations
internal/parser/            .clit format parser (entry builder, assert/capture parsing)
internal/runner/            Command execution via sh -c, captures stdout/stderr/exit/duration
internal/assert/            Predicate evaluation engine (queries + predicates + negation)
pkg/types/                  Shared types: Entry, Assert, Capture, File
examples/*.clit             User-facing usage examples (also validated via `make examples`)
test/e2e/syntax/            E2E tests for .clit syntax (asserts, captures)
test/e2e/output/            E2E tests for header/footer/summary output and output v2
test/e2e/options/           E2E tests for CLI flags (--var, --parallel, -v)
test/e2e/resolve/           E2E tests for file discovery (recursive, glob, skip warnings)
test/_fixtures/             Shared test fixtures (e.g. intentionally failing .clit files)
```

## Conventions

- TDD: write/update `*_test.go` before or alongside implementation.
- `test/e2e/` contains all E2E tests of the clit tool itself — keep them green via `make e2e`.
- `examples/` contains user-facing usage demonstrations — validated via `make examples`.
- Parser is hand-rolled (no parser generator). Entries separated by blank lines; sections by `[Name]` headers.
- Values in asserts: `"quoted"` strings, `/regex/` literals, or bare tokens. See `unquoteValue` in parser.
- Runner always uses `sh -c`; commands are single-line only (multi-line is planned but not implemented).
- Variable substitution (`{{name}}`) happens in `cmd/clit/main.go` before parsing — it's a simple string replace, not part of the parser.
- File discovery is recursive by default (`--no-recursive` to disable). Non-`.clit` files are skipped with a warning to stderr.
- Glob patterns in arguments (quoted, e.g. `"examples/*.clit"`) are expanded by clit itself, not the shell. The original pattern is preserved in the header output.
