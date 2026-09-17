# File-level `@var` directive — design

Issue: https://github.com/sleipi/cli-t/issues/60

## Problem

`--var NAME=VALUE` feeds `{{NAME}}` but is global to the run. Sibling
`.clitest` files that differ only in one value (a directory name, a
tool version, …) end up repeating that value on every line that needs
it. Renaming the shared value means editing every occurrence, in every
file, and it is easy to miss one silently.

## Goal

A frontmatter directive, `@var KEY=VALUE`, that sets a value scoped to
the file it's declared in, feeding the existing `{{...}}` template
system. Sibling files then differ only in their frontmatter line.

## Non-goals

- **Entry-level `@var`.** Would require moving `{{...}}` substitution
  from a single whole-file text pass (before parsing) to a per-entry
  pass (after parsing), since two entries in one file could need the
  same placeholder resolved differently — a plain string replace over
  the raw file text can't do that. No use case for this is described
  in the issue; out of scope here, open a follow-up issue if needed.
- **Rejecting `@var` placed after the frontmatter.** Unknown
  directives in entry position are already silently ignored today
  (`interpretEntryDirectives` has no `default` error case) — this is
  existing, consistent behavior for every directive, not something
  this feature special-cases.
- **`@env`/`$VAR` integration.** `@env` feeds child-process environment
  variables (`$VAR` in commands), a separate mechanism from `{{...}}`.
  `@var` does not touch it.

## Resolution order

```
--var (CLI)  >  [captures]  >  @var (file)
```

`--var` is resolved today via a single `vars.Substitute(content, cliVars)`
pass over the raw file text, before parsing — unchanged by this
feature. Whatever `{{...}}` placeholders that pass didn't resolve stay
literal in the parsed `entry.Command` string.

`[captures]` are resolved at runtime, per entry, via
`vars.SubstituteCaptures(entry.Command, captures)` — the only place
`{{...}}` is substituted after parsing today. This already only
touches `entry.Command`, not asserts/body — an existing limitation,
unchanged here.

`@var` slots in at that same runtime step: the map passed to
`SubstituteCaptures` becomes `merge(fileVars, captures)`, with
`captures` overwriting `fileVars` on key collision. Because `--var`
already consumed matching placeholders at load time (before this step
ever runs), and `captures` overwrite `@var` in the merge, the net
effect is exactly `--var > captures > @var` with no change to the
load-time substitution pass and no per-entry re-architecture.

## Changes

**`internal/types/types.go`**
- `FileDirectives.Var map[string]string` (mirrors `.Env`). No
  `EntryDirectives.Var`.

**`internal/parser/directives.go`**
- `interpretFileDirectives`: new `case "var"`. Same malformed-value
  check as `@env` (`KEY=VALUE`, non-empty key, `DirectiveError` if no
  `=`). Additionally: a second `@var` for the same key within one
  frontmatter block is a hard `DirectiveError` (not last-write-wins,
  unlike `@env`, since a duplicate file-level default is almost
  certainly a copy-paste mistake worth surfacing).
- `interpretEntryDirectives`: no change.

**`internal/vars/vars.go`**
- `MergeCaptures(fileVars, captures map[string]string) map[string]string`
  — returns a new map, `captures` entries win over `fileVars` on
  collision. Used at all 5 existing `SubstituteCaptures` call sites
  (`cmd/clitest/run.go` ×2, `internal/executor/executor.go` ×2,
  `internal/executor/background.go` ×1) in place of the bare
  `captures` map.

**`SPEC.md`**
- Variables → priority list: insert `@var` between `[captures]` and
  environment-variable expansion.
- Directives → Available Directives table: add `@var` row, scope
  `file` only.
- New `#### @var` subsection (pattern matches `#### @env`), with a
  worked example mirroring the sibling-file case from the issue.

## Testing

- **Unit** — `internal/parser/directives_test.go`: parses `@var`,
  rejects malformed (no `=`), rejects duplicate key.
  `internal/vars/vars_test.go`: `MergeCaptures` precedence.
- **e2e** — `test/e2e/var/` (pattern: `test/e2e/env/`):
  - `it_sets_file_level_var.clitest`
  - `it_var_available_to_all_entries.clitest`
  - `it_captures_override_var.clitest`
  - `it_cli_var_overrides_var.clitest`
  - `it_rejects_duplicate_var.clitest`
  - `it_rejects_malformed_var.clitest`
- **examples** — `examples/09_var.clitest`: the sibling-file scenario
  from the issue (one shared value driving several paths/commands via
  one frontmatter line).

## Error handling

Both new failure modes (malformed value, duplicate key) surface as
`*parser.DirectiveError` — already handled end-to-end in
`cmd/clitest/run.go` (formatted with file:line prefix, same path as
existing `@env`/`@timeout` validation errors). No new error-handling
code needed there.
