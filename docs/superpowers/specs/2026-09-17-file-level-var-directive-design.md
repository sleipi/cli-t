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

`@var` slots into that same runtime step, but even more directly than
originally planned: the `captures` map is a single instance, created
once per file run (`cmd/clitest/run.go:47`, `captures := map[string]string{}`)
and threaded by reference through every executor function that reads
or writes it — all 5 `SubstituteCaptures` call sites use this same
instance. Seeding that map with `f.Directives.Var` at creation (a
copy, so the original `FileDirectives.Var` is never mutated) means
every one of those call sites picks up `@var` values automatically,
with zero changes inside the executor package. Real captures
(`captures[c.Name] = val`, written as entries execute) then overwrite
the seeded value for the same key on the same map — ordinary map
write semantics, no merge function needed. Net effect is exactly
`--var > captures > @var`, with no change to the load-time
substitution pass and no per-entry re-architecture.

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
- `SeedCaptures(fileVars map[string]string) map[string]string` —
  returns a fresh copy of `fileVars` (empty map if `fileVars` is nil),
  never the original map. Used once, in place of the literal
  `map[string]string{}` at `cmd/clitest/run.go:47`, to initialize the
  `captures` map that's threaded through the rest of the run.

**`cmd/clitest/run.go` / `cmd/clitest/root.go`**
- `runEntries`, `runEntriesVerbose`, `runEntriesCompact` each gain a
  `fileVars map[string]string` parameter, threaded from
  `parsed.Directives.Var` in `root.go` (where `parsed` is the
  `*types.File` returned by `loadAndParse`) down to the `captures :=`
  initialization in `runEntries`.

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
  `internal/vars/vars_test.go`: `SeedCaptures` (copies, doesn't alias
  the input map; nil input yields empty map).
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
