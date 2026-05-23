# Roadmap

## Completed

- [x] TextMate grammar for `.clitest` syntax highlighting — VS Code extension + IntelliJ/Sublime support via TextMate Bundles. [#30](https://github.com/sleipi/cli-t/issues/30)
- [x] `--silent` / `-s` — Suppress all output except the summary line and failure details. Warnings are suppressed but their count is included in the summary. Three output levels: silent (`-s`) / normal (default) / verbose (`-v`). [#38](https://github.com/sleipi/cli-t/issues/38)
- [x] `@workdir ./path` — Run command in specific directory. Supported at file-level (frontmatter) and entry-level. Entry-level overrides file-level. Relative paths resolve relative to the `.clitest` file's directory. See #21.
- [x] Auto-exec for `EXIT NEVER` background processes — Automatically inserts `exec` for simple commands to ensure signals reach the target process directly on all `/bin/sh` implementations (bash, dash, busybox ash). Handles env-prefix commands (`ENV=val exec ./cmd`). Complex commands with shell operators are left unchanged; users can manually prefix `exec` if needed.
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
- [ ] `--json` — Output test results as structured JSON for programmatic consumption [#23](https://github.com/sleipi/cli-t/issues/23)
- [ ] `--markdown` — Output test results as structured markdown for AI consumption [#24](https://github.com/sleipi/cli-t/issues/24)
- [ ] `@timeout MS` — Extend to regular entries (kill process after MS milliseconds). Currently only works for `EXIT NEVER` entries. [#25](https://github.com/sleipi/cli-t/issues/25)
- [ ] `@retry N` — Retry on failure N times [#26](https://github.com/sleipi/cli-t/issues/26)
- [ ] `@env KEY=VALUE` — Set env vars for entry [#27](https://github.com/sleipi/cli-t/issues/27)
- [ ] `@workdir ./path` — Run command in specific directory [#28](https://github.com/sleipi/cli-t/issues/28)
- [ ] `@shell NAME` — Override the default shell (`sh`) used to execute commands [#29](https://github.com/sleipi/cli-t/issues/29) (e.g. `bash`, `zsh`, `podman compose exec <container> <command>`, `podman run -it <container> <command>`)
- [ ] IntelliJ Plugin — Run Configuration for `.clitest` files [#41](https://github.com/sleipi/cli-t/issues/41)
- [ ] Release for Homebrew [#31](https://github.com/sleipi/cli-t/issues/31)
- [ ] Publish to Alpine Repository [#32](https://github.com/sleipi/cli-t/issues/32)
- [ ] Publish to Debian Repository [#33](https://github.com/sleipi/cli-t/issues/33)
- [ ] Website + Docs — Documentation website with domain [#34](https://github.com/sleipi/cli-t/issues/34)
- [ ] go install github.com/sleipi/cli-t [#35](https://github.com/sleipi/cli-t/issues/35)
- [ ] Full-text assert with linebreaks — Multi-line string assertions in `[Asserts]` section [#36](https://github.com/sleipi/cli-t/issues/36)
- [ ] JSONPath assertions — `json` query type for structured output assertions [#37](https://github.com/sleipi/cli-t/issues/37)
- [ ] Output v3 - Improve test output formatting and usability (display pid of each background process, align formatting)
- [ ] `--junit FILE` — Write a JUnit XML report to the given file path for CI integration
- [ ] `@hook` —
- [ ] Shell completion (bash/zsh/fish) via cobra completion subcommand
- [ ] Register Domain
- [ ] Publish VS Code extension to Marketplace (requires publisher account)
