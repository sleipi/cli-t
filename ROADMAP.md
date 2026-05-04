# Roadmap

## Completed

- [x] Run multiple files at once — pass multiple paths or directories as arguments
- [x] Recursive file discovery — directories are scanned recursively by default
- [x] Parallelism — Files run concurrently (default: 8 workers), entries within a file run sequentially. `--parallel N` to configure, `--no-parallel` to disable.
- [x] Header/Footer — Shows version, paths, options, and execution duration (`took:`) in summary
- [x] Glob support — Quoted glob patterns (e.g. `"examples/*.clit"`) are expanded by clit, preserving the pattern in header output
- [x] Output v2 — Compact progress bars as default output; verbose (`-v`) becomes the detailed per-entry display. TTY-aware with dynamic ANSI updates, static fallback for non-TTY. Progress counter `(n/total)`, per-file timing, and entry subtitle (comment or command) shown while running.
- [x] Multi-line commands — Support commands that span multiple lines (trailing `\` continuation)
- [x] GitHub Actions to run CI (unit + e2e)

## Planned

- [ ] `--fail-fast` — Stop execution on the first test failure instead of running all entries
- [ ] `--no-color` — Disable ANSI color codes in output (useful for CI or piping)
- [ ] `--json` — Output test results as structured JSON for programmatic consumption
- [ ] `--mardown` — Output test results as structured markdown for AI consumption
- [ ] `--junit FILE` — Write a JUnit XML report to the given file path for CI integration
- [ ] `--timeout MS` — Set a maximum execution time per command (kills the process after MS milliseconds)
- [ ] `--filter GLOB` — Only run entries whose comment matches the given glob pattern
- [ ] `--shell NAME` — Override the default shell (`sh`) used to execute commands (e.g. `bash`, `zsh`)
- [ ] Dependencies between entries — Allow entries to declare dependencies on other entries or captures
- [ ] Add file parser Plugin for Intellj