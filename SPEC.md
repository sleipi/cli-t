# `.clit` File Format Specification

Version: 0.2-draft

## Overview

A `.clit` (CLI Test) file is a declarative test specification for shell commands. Each file contains one or more **entries**, separated by blank lines. Each entry specifies a command to run, the expected exit code, and optional assertions against the command's output.

## File Structure

```
[entry]

[entry]

[entry]
```

Entries are separated by one or more blank lines.

---

## Entry Structure

An entry consists of the following parts, in order:

```
[comment]
<command>
EXIT <code>
[body]
[sections]
```

### 1. Comment (optional)

One or more lines starting with `#`. Describes the test case.

```
# Verify that the help flag shows usage info
# and exits cleanly
```

Comments MUST appear before the command. Comments bind to the entry that follows them.

### 2. Command (required)

A single line containing the shell command to execute. The command is run via `sh -c "<command>"`.

```
echo "hello world"
```

Multi-line commands use trailing `\` (backslash continuation):

```
curl -s https://api.example.com/health \
  -H "Accept: application/json"
```

> **Future**: Multi-line command support is not yet implemented. Currently commands must be a single line.

### 3. EXIT (optional)

Declares the expected exit code. Defaults to `0` if omitted.

```
EXIT 0
EXIT 1
EXIT 127
```

### 4. Body (optional)

Lines after `EXIT` (or after the command if `EXIT` is omitted) that don't start a section are treated as the **expected stdout body**. This is an exact match assertion — the command's stdout (with trailing newline stripped) must equal the body lines joined by `\n`.

```
echo "hello"
EXIT 0
hello
```

For body content that contains blank lines, use fenced blocks:

````
printf "a\n\nb"
EXIT 0
```
a

b
```
````

### 5. Sections (optional)

Sections start with a header in square brackets. Available sections:

- `[Asserts]` — explicit assertions
- `[Captures]` — variable captures
- `[Options]` — entry-level options (planned)

---

## Sections

### `[Asserts]`

Each line in the `[Asserts]` section is an assertion with the format:

```
<query> [not] <predicate> [value]
```

#### Queries

| Query        | Description                              | Value type |
|-------------|------------------------------------------|-----------|
| `stdout`    | Full stdout (trailing `\n` stripped)      | string    |
| `stderr`    | Full stderr (trailing `\n` stripped)      | string    |
| `exit`      | Exit code                                 | integer   |
| `line N`    | Nth line of stdout (1-indexed)            | string    |
| `lineCount` | Number of non-empty lines in stdout       | integer   |
| `duration`  | Command execution time in milliseconds    | integer   |

> **Planned queries**: `header "Name"`, `json "$.path"`, `wordCount`, `field N` (whitespace-split)

#### Predicates

| Predicate     | Arity | Description                    | Applicable types |
|--------------|-------|--------------------------------|-----------------|
| `contains`   | 1     | Substring match                | string          |
| `startsWith` | 1     | Prefix match                   | string          |
| `endsWith`   | 1     | Suffix match                   | string          |
| `matches`    | 1     | Regex match (Go `regexp`)      | string          |
| `isEmpty`    | 0     | Value is empty string          | string          |
| `==`         | 1     | Equality (numeric-aware)       | string, integer |
| `!=`         | 1     | Inequality                     | string, integer |
| `>`          | 1     | Greater than                   | integer         |
| `>=`         | 1     | Greater than or equal          | integer         |
| `<`          | 1     | Less than                      | integer         |
| `<=`         | 1     | Less than or equal             | integer         |

> **Planned predicates**: `includes` (JSON array), `exists`, `isNumber`, `count` (regex match count)

#### Negation

Any predicate can be negated with the `not` keyword:

```
stdout not contains "error"
stderr not isEmpty
```

#### Values

Values can be:
- Quoted strings: `"hello world"` — quotes are stripped
- Regex literals: `/\d{4}-\d{2}-\d{2}/` — slashes are stripped
- Bare values: `42`, `hello` — used as-is

```
[Asserts]
stdout contains "expected output"
stdout matches /\d+\.\d+\.\d+/
lineCount == 3
line 1 startsWith "Usage:"
stderr isEmpty
duration < 5000
exit == 0
```

---

### `[Captures]`

Captures extract values from command output and store them as variables for use in subsequent entries.

Format:

```
<name>: <query>
```

Example:

```
echo "abc-123"
EXIT 0
[Captures]
id: stdout
```

The captured value can then be used in later entries as `{{id}}`.

> **Planned**: Regex capture groups: `token: stdout regex /token=(\w+)/`

---

### `[Options]` (planned)

Entry-level options that control execution behavior.

```
[Options]
timeout: 5000
workdir: /tmp
shell: bash
env: FOO=bar
env: BAZ=qux
retry: 3
retry-interval: 1000
skip: $CI != "true"
```

| Option           | Type    | Default   | Description                          |
|-----------------|---------|-----------|--------------------------------------|
| `timeout`       | integer | 30000     | Max execution time in ms             |
| `workdir`       | string  | `.`       | Working directory for the command     |
| `shell`         | string  | `sh`      | Shell to use (`sh`, `bash`, `zsh`)   |
| `env`           | string  | —         | Additional env var (repeatable)      |
| `retry`         | integer | 0         | Number of retries on failure         |
| `retry-interval`| integer | 1000      | Delay between retries in ms          |
| `skip`          | string  | —         | Condition to skip this entry         |

---

## Variables

### Template Syntax

Variables use double-brace syntax: `{{name}}`

```
echo "{{greeting}}"
EXIT 0
{{greeting}}
```

Variables are resolved from (in priority order):
1. `--var NAME=VALUE` CLI flags
2. `[Captures]` from previous entries
3. Environment variables (via `$VAR` or `${VAR}` syntax)

### Environment Variable Expansion

Standard shell-style `$VAR` and `${VAR}` are expanded from the process environment:

```
echo $HOME
EXIT 0
[Asserts]
stdout == "$HOME"
```

> Note: `$VAR` in the command itself is expanded by the shell. In body/asserts, it's expanded by clit before comparison.

---

## File-Level Directives (planned)

At the top of a `.clit` file, before any entries:

```
@name "Authentication Tests"
@timeout 10000
@shell bash
@setup echo "setup"
@teardown echo "cleanup"
@env API_URL=http://localhost:3000
```

| Directive    | Description                                         |
|-------------|-----------------------------------------------------|
| `@name`     | Human-readable test suite name                      |
| `@timeout`  | Default timeout for all entries                     |
| `@shell`    | Default shell for all entries                       |
| `@setup`    | Command to run before the first entry               |
| `@teardown` | Command to run after the last entry (always)        |
| `@env`      | Set environment variable for all entries            |
| `@require`  | Prerequisite command that must succeed (else skip)  |

---

## Full Example

```clit
# Login and capture token
curl -s -X POST http://localhost:3000/login \
  -d '{"user":"admin","pass":"secret"}'
EXIT 0
[Asserts]
stdout matches /token/
[Captures]
token: stdout

# Use token to fetch profile
curl -s http://localhost:3000/me -H "Authorization: Bearer {{token}}"
EXIT 0
[Asserts]
stdout contains "admin"
line 1 startsWith "{"
```

---

## CLI Interface

```
clit [options] <path...>
```

### Arguments

`<path...>` — one or more `.clit` files, directories, or glob patterns (quoted). Directories are scanned recursively for `*.clit` files by default. Glob patterns (containing `*`, `?`, or `[`) are expanded by clit itself. Non-`.clit` files passed as arguments are skipped with a warning.

### Options

| Flag              | Description                                |
|------------------|--------------------------------------------|
| `-v`             | Verbose: show stdout/stderr for passing tests |
| `--var NAME=VAL` | Set template variable (repeatable)         |
| `--no-recursive` | Disable recursive directory scanning       |
| `--parallel N`   | Max parallel file executions (default: 8)  |
| `--no-parallel`  | Disable parallel execution (sequential)    |

### Exit Codes

| Code | Meaning                |
|------|------------------------|
| 0    | All tests passed       |
| 1    | One or more tests failed |
| 2    | Usage error / invalid input |

### Output Format

When invoked, clit prints a header block, test results, and a summary footer:

```
clit v0.1.0
  path:     examples/ (6 file(s) loaded)
  path:     tests/ (3 file(s) loaded)
  parallel: 8
  verbose:  on
  vars:     FOO, BAR

▶ 01_basic.clit
  ✓ echo "hello world" (exit=0, 1 asserts)

━━━ Summary ━━━
  pass: 3
  took: 164ms
```

**Header rules:**
- `path:` — one line per argument, showing file count. Suffix `- no-recursive` when `--no-recursive` is active.
- `parallel: N` — shown when parallel is active. Replaced by `no-parallel` when `--no-parallel` is set.
- `verbose: on` — only shown when `-v` is active.
- `vars:` — only shown when `--var` flags are set.

**Footer:**
- `took:` — execution duration. Format: `<1s` → milliseconds (`230ms`), `>=1s` → seconds (`1.23s`).

### Parallel Execution

Files are executed concurrently using a worker pool (default: 8 workers). Entries within a file always run sequentially to preserve capture dependencies. Output is buffered per file and printed in deterministic order (sorted file paths).

---

## Grammar (informal)

```
file       = directive* (entry separator)* entry?
separator  = BLANK_LINE+
entry      = comment? command EXIT? body? section*
comment    = (HASH TEXT NEWLINE)+
command    = TEXT NEWLINE
EXIT       = "EXIT" SPACE INTEGER NEWLINE
body       = unfenced_body | fenced_body
unfenced_body = (TEXT NEWLINE)+           # terminated by blank line or section
fenced_body   = "```" NEWLINE (ANY NEWLINE)* "```" NEWLINE
section    = section_header section_body
section_header = "[" NAME "]" NEWLINE
section_body   = (TEXT NEWLINE)*          # terminated by blank line or section

assert     = query SPACE ["not" SPACE] predicate [SPACE value]
query      = "stdout" | "stderr" | "exit" | "lineCount" | "duration" | "line" SPACE INTEGER
predicate  = "contains" | "startsWith" | "endsWith" | "matches" | "isEmpty" | OP
OP         = "==" | "!=" | ">" | ">=" | "<" | "<="
value      = QUOTED | REGEX | BARE
QUOTED     = '"' [^"]* '"'
REGEX      = '/' [^/]* '/'
BARE       = \S+

capture    = NAME ":" SPACE query
```

---

## Design Principles

1. **Readable first** — A `.clit` file should be understandable without documentation
2. **Minimal ceremony** — Common cases (exit 0, exact body match) need minimal syntax
3. **Progressive disclosure** — Simple tests are simple; complex tests add sections
4. **Hermetic by default** — Each entry is independent unless explicitly linked via captures
5. **Fast feedback** — Colored output, parallel execution, duration tracking
