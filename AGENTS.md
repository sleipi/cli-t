# AGENTS.md

## Project

`clit` (CLI Test) — a Go CLI tool that runs declarative `.clit` test files against shell commands. Single binary, zero dependencies.

- Module: `github.com/ronny/clit`
- Binary output: `./clit` (repo root)
- Spec: `SPEC.md` — the authoritative syntax reference; keep it in sync with code changes.

## Tech Stack

* go-lang
* write Code using TTL
* always test the App with a CLIT Test Case

## Commands

```bash
make build        # go build -o clit ./cmd/clit/
make test         # go test ./...
make e2e          # build + run ./clit examples/
make all          # test + e2e (use this to verify changes)
make self-test    # build + ./clit examples/99_self_test.clit
```

Always run `make all` after changes — unit tests alone won't catch parser/CLI regressions.

## Package Layout

```
cmd/clit/main.go        CLI entrypoint, flag parsing, colored output, file discovery, parallel execution
cmd/clit/main_test.go   Unit tests for resolveFiles, glob support
internal/parser/        .clit format parser (entry builder, assert/capture parsing)
internal/runner/        Command execution via sh -c, captures stdout/stderr/exit/duration
internal/assert/        Predicate evaluation engine (queries + predicates + negation)
pkg/types/              Shared types: Entry, Assert, Capture, File
examples/*.clit         E2E test fixtures (also used as self-tests)
examples/header/        Tests for header/footer output
examples/options/       Tests for CLI flags (--var, --parallel, -v)
examples/captures/      Tests for [Captures] section
examples/nested/        Tests for recursive file discovery
```

## Conventions

- TDD: write/update `*_test.go` before or alongside implementation.
- `examples/99_self_test.clit` runs `./clit` against other example files — keep it green.
- Parser is hand-rolled (no parser generator). Entries separated by blank lines; sections by `[Name]` headers.
- Values in asserts: `"quoted"` strings, `/regex/` literals, or bare tokens. See `unquoteValue` in parser.
- Runner always uses `sh -c`; commands are single-line only (multi-line is planned but not implemented).
- Variable substitution (`{{name}}`) happens in `cmd/clit/main.go` before parsing — it's a simple string replace, not part of the parser.
- File discovery is recursive by default (`--no-recursive` to disable). Non-`.clit` files are skipped with a warning to stderr.
- Glob patterns in arguments (quoted, e.g. `"examples/*.clit"`) are expanded by clit itself, not the shell. The original pattern is preserved in the header output.
