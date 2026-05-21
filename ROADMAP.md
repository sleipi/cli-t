# Roadmap

## Completed

- [x] `[Finally]` section + `later` assert modifier — Send a signal to background processes at file-end and assert exit code + output. `later` keyword defers assert evaluation to file-end. Execution order: entries → later asserts → [Finally] LIFO → @defer LIFO. See #19.
- [x] `[Prompts]` — Interactive prompt/response section: match stdout patterns and send responses via stdin. Pipe-based (no PTY). Supports substring and regex matching, multiplier syntax, ambiguity detection, and unmatched prompt failure. Default timeout 30s.
- [x] `--no-color` — Disable ANSI color codes in output. Also respects `NO_COLOR` env var (https://no-color.org/) and auto-disables when stdout is not a TTY.
- [x] `--fail-fast` — Stop execution on the first test failure instead of running all entries
- [x] Refactor `cmd/clitest/` package structure — Extracted display, resolve, filter, vars, and executor logic into dedicated `internal/` packages. Reduced `cmd/clitest/` from 17 to 7 files.
- [x] Linting — Introduced `golangci-lint` with CI integration and resolved all issues
- [x] Restructure E2E tests to behavior-driven style — files named `it_<describes_behavior>.clitest` (e.g. `it_does_not_execute_skipped_entries.clitest`). One behavior per file, multiple entries allowed when orchestration is needed.
- [x] Better CLI Help and Usage output (using `cobra`)
- [x] Run multiple files at once — pass multiple paths or directories as arguments
- [x] Recursive file discovery — directories are scanned recursively by default
- [x] Parallelism — Files run concurrently (default: 8 workers), entries within a file run sequentially. `--parallel N` to configure, `--no-parallel` to disable.
- [x] Header/Footer — Shows version, paths, options, and execution duration (`took:`) in summary
- [x] Glob support — Quoted glob patterns (e.g. `"examples/*.clitest"`) are expanded by clit, preserving the pattern in header output
- [x] Output v2 — Compact progress bars as default output; verbose (`-v`) becomes the detailed per-entry display. TTY-aware with dynamic ANSI updates, static fallback for non-TTY. Progress counter `(n/total)`, per-file timing, and entry subtitle (comment or command) shown while running.
- [x] Multi-line commands — Support commands that span multiple lines (trailing `\` continuation)
- [x] GitHub Actions to run CI (unit + e2e)
- [x] Directives — Generic `@directive` system with frontmatter (`---` block) for file-level and entry-level metadata
- [x] `@group` — Tag entries/files with space-separated tags for filtering (`--group TAG`, `--exclude-group TAG`, OR logic)
- [x] `@skip` — Skip entries/files with optional reason, displayed as SKIP in output with skip count in summary
- [x] Background processes — `EXIT NEVER`, `@poll`, `@defer`, `pid` capture: start long-running commands, poll asserts until pass/timeout, cleanup via defer (LIFO)

## Planned
- [ ] Output v3 - Improve test output formatting and usability (display pid of each background process, align formatting)
- [ ] `--json` — Output test results as structured JSON for programmatic consumption
- [ ] `--mardown` — Output test results as structured markdown for AI consumption
- [ ] `--junit FILE` — Write a JUnit XML report to the given file path for CI integration
- [ ] `@timeout MS` — Extend to regular entries (kill process after MS milliseconds). Currently only works for `EXIT NEVER` entries.
- [ ] `@retry N` — Retry on failure N times
- [ ] `@env KEY=VALUE` — Set env vars for entry
- [ ] `@workdir ./path` — Run command in specific directory
- [ ] `@hook` - 
- [ ] `@shell NAME` — Override the default shell (`sh`) used to execute commands (e.g. `bash`, `zsh`, `podman compose exec <container> <command>`, `podman run -it <container> <command>`)
- [ ] Add file parser Plugin for Intellj
- [ ] Release for Homebrew
- [ ] Shell completion (bash/zsh/fish) via cobra completion subcommand (`cobra` support's it out-of-the-box)
- [ ] Publish to Alpine Repository
- [ ] Publish to Debian Repository
- [ ] Publish to Home Brew
- [ ] Register Domain + Docs
- [ ] go install github.com/sleipi/cli-t
- [ ] Website + Docs — Documentation website with domain, hosting (e.g. Hugo/Docusaurus + GitHub Pages), syntax reference, and getting-started guide
- [ ] Full-text assert with linebreaks — Multi-line string assertions in `[Asserts]` section (heredoc or fenced-block syntax for values containing newlines)
- [ ] JSONPath assertions — `json` query type for structured output assertions (e.g. `stdout.json $.items[0].name equals "foo"`). Scope TBD.

## Bugs

## Syntax Sketches

### `[Prompts]` — Interactive prompt/response

Matches stdout/stderr patterns at runtime and writes responses to stdin. Syntax: `<pattern> => <response>`.

- **Pattern:** `"substring"` (contains-match) or `/regex/` (regex-match)
- **Response:** `"quoted string"` — written to stdin + newline
- **Multiplier:** `* N` — allows the same prompt to match N times
- **Semantics:** async stdout/stderr reading, first match wins, response sent on match
- **Failure conditions:**
  - Unmatched prompt at process end → FAIL ("Prompt X was never matched")
  - Two patterns match simultaneously → FAIL (ambiguity error)
  - Process blocks on stdin with no matching prompt defined → FAIL (via `@timeout`)
- **Requires:** `@timeout` directive

```clitest
# Interactive Symfony Console command
@timeout 5000
php bin/console app:create-user
EXIT 0
[Prompts]
"Enter username:" => "alice"
"Enter email:" => "alice@example.com"
/Confirm .* \[yes\]/ => "yes"
[Asserts]
stdout contains "User created"
```

```clitest
# Prompt that appears multiple times
@timeout 3000
./setup.sh
EXIT 0
[Prompts]
"Continue?" => "yes" * 3
"Enter name:" => "Alice"
```

make all not update lines
```
RUN [>         ] - verbose.clitest (0/2) 23ms
    Verbose shows stdout on passing tests
RUN [>         ] - output.clitest (0/7) 23ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 23ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [>         ] - verbose.clitest (0/2) 23ms
    Verbose shows stdout on passing tests
RUN [>         ] - output.clitest (0/7) 23ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 23ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [>         ] - verbose.clitest (0/2) 23ms
    Verbose shows stdout on passing tests
RUN [>         ] - output.clitest (0/7) 23ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 25ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [>         ] - verbose.clitest (0/2) 25ms
    Verbose shows stdout on passing tests
RUN [>         ] - output.clitest (0/7) 25ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 26ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [>         ] - verbose.clitest (0/2) 26ms
    Verbose shows stdout on passing tests
RUN [>         ] - output.clitest (0/7) 26ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 26ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [>         ] - verbose.clitest (0/2) 26ms
    Verbose shows stdout on passing tests
RUN [>         ] - output.clitest (0/7) 26ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 26ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [>         ] - verbose.clitest (0/2) 26ms
    Verbose shows stdout on passing tests
RUN [>         ] - output.clitest (0/7) 26ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 26ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 26ms
    Verbose shows stdout on passing tests
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 26ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 26ms
    Without verbose, passing tests don't show stdout
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 27ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 27ms
    Without verbose, passing tests don't show stdout
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [>         ] - parallel.clitest (0/2) 27ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 27ms
    Without verbose, passing tests don't show stdout
RUN [>         ] - output.clitest (0/7) 27ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 27ms
    Parallel flag sets worker count
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 27ms
    Without verbose, passing tests don't show stdout
RUN [>         ] - output.clitest (0/7) 27ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 28ms
    No-parallel flag disables parallelism
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 28ms
    Without verbose, passing tests don't show stdout
RUN [>         ] - output.clitest (0/7) 28ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 29ms
    No-parallel flag disables parallelism
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 29ms
    Without verbose, passing tests don't show stdout
RUN [>         ] - output.clitest (0/7) 29ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 29ms
    No-parallel flag disables parallelism
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 29ms
    Without verbose, passing tests don't show stdout
RUN [>         ] - output.clitest (0/7) 29ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 30ms
    No-parallel flag disables parallelism
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 30ms
    Without verbose, passing tests don't show stdout
RUN [>         ] - output.clitest (0/7) 30ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 30ms
    No-parallel flag disables parallelism
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 30ms
    Without verbose, passing tests don't show stdout
RUN [>         ] - output.clitest (0/7) 30ms
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 30ms
    No-parallel flag disables parallelism
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 30ms
    Without verbose, passing tests don't show stdout
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 30ms
    No-parallel flag disables parallelism
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 30ms
    Without verbose, passing tests don't show stdout
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 30ms
    No-parallel flag disables parallelism
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 30ms
    Without verbose, passing tests don't show stdout
OK  [==========] - var_sub.clitest (1/1) took 9ms
RUN [=====>    ] - parallel.clitest (1/2) 31ms
    No-parallel flag disables parallelism
OK  [==========] - var.clitest (1/1) took 16ms
RUN [=====>    ] - verbose.clitest (1/2) 31ms
    Without verbose, passing tests don't show stdout
OK  [==========] - var_sub.clitest (1/1) took 9ms
```

# Background process starten
@timeout 5000
$ php -S localhost:8080 &
wait-for stdout /Development server/

# Assertions gegen das laufende System
$ curl -s http://localhost:8080/index.php
stdout = "Hello World"

# hier brauchen wir noch was am ende
@kill pid von oben :)