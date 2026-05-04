# Roadmap

## Completed

- [x] Run multiple files at once — pass multiple paths or directories as arguments
- [x] Recursive file discovery — directories are scanned recursively by default
- [x] Parallelism — Files run concurrently (default: 8 workers), entries within a file run sequentially. `--parallel N` to configure, `--no-parallel` to disable.
- [x] Header/Footer — Shows version, paths, options, and execution duration (`took:`) in summary
- [x] Glob support — Quoted glob patterns (e.g. `"examples/*.clit"`) are expanded by clit, preserving the pattern in header output

## Planned

- [ ] `--fail-fast` — Stop execution on the first test failure instead of running all entries
- [ ] `--no-color` — Disable ANSI color codes in output (useful for CI or piping)
- [ ] `--json` — Output test results as structured JSON for programmatic consumption
- [ ] `--junit FILE` — Write a JUnit XML report to the given file path for CI integration
- [ ] `--timeout MS` — Set a maximum execution time per command (kills the process after MS milliseconds)
- [ ] `--filter GLOB` — Only run entries whose comment matches the given glob pattern
- [ ] `--shell NAME` — Override the default shell (`sh`) used to execute commands (e.g. `bash`, `zsh`)
- [ ] Dependencies between entries — Allow entries to declare dependencies on other entries or captures
- [ ] Multi-line commands — Support commands that span multiple lines
