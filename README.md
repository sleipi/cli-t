# CLI-Testing — Declarative Test Runner for Shell Commands

**Stop writing fragile bash scripts to test your CLI tools.** Write what you expect, and let CLI-Testing do the rest.

CLI-Testing is a single-binary, zero-dependency test runner for shell commands. You describe your expected outputs in simple, readable `.clit` files — no test framework, no scripting, no boilerplate. If your tool runs in a terminal, CLI-Testing can test it.

Heavily inspired by [Hurl](https://hurl.dev) — which does the same for HTTP requests — CLI-Testing brings that same declarative, readable approach to testing CLI commands.

---

## Quick Example

```
# Verify JSON output from our API tool
my-cli export --format json
EXIT 0
[Asserts]
stdout contains "version"
stdout matches /"\d+\.\d+\.\d+"/
lineCount == 1
```

That's a complete test. No setup, no teardown, no imports. Just: command, expected exit code, assertions.

---

## Why CLI-Testing?

As developers, we constantly build CLI tools, scripts, and pipelines. Testing them usually means one of:

- **Bash scripts** that grow into unmaintainable monsters with nested `if`s and string comparisons
- **Heavy test frameworks** that require a runtime, dependencies, and ceremony just to check "did my command print the right thing?"
- **Manual testing** — running commands by hand and eyeballing the output (we've all been there)

CLI-Testing gives you a middle ground: **tests that read like documentation** and run in milliseconds.

```
# Health check returns OK
curl -s http://localhost:8080/health
EXIT 0
ok
```

You read it, you understand it, your teammates understand it. Done.

---

## Features

- **Exact body matching** — assert full stdout output line-by-line
- **Rich assertions** — `contains`, `startsWith`, `endsWith`, `matches` (regex), `lineCount`, line-based queries
- **Negation** — `stdout not contains "error"`
- **Directives** — `@skip`, `@group`, `@name`, `@depends` for organizing and controlling test flow
- **Captures** — extract values from output and reuse them across entries
- **Variables** — pass `--var key=value` from the CLI, use `{{key}}` in your tests
- **Parallel execution** — run test files concurrently with `--parallel`
- **Recursive discovery** — point it at a directory and it finds all `.clit` files
- **Glob support** — `clit "tests/**/*.clit"`
- **Zero dependencies** — single Go binary, runs anywhere

---

## Installation

```bash
go install github.com/sleipi/clit/cmd/clit@latest
```

Or build from source:

```bash
git clone https://github.com/sleipi/clit.git
cd clit
make build
# ./clit is ready to use
```

---

## Usage

```bash
# Run all tests in a directory (recursive)
clit test/

# Run a specific file
clit test/e2e/syntax/asserts.clit

# Use a glob pattern
clit "test/**/*.clit"

# Pass variables
clit --var host=localhost --var port=8080 test/

# Verbose output
clit -v test/

# Parallel execution
clit --parallel test/
```

---

## .clit File Anatomy

```
# Optional comment describing the test
@group smoke
my-command --flag value
EXIT 0
expected output line 1
expected output line 2
[Asserts]
stdout contains "success"
stdout matches /took \d+ms/
lineCount == 2
```

| Part         | Required | Description                                    |
|--------------|----------|------------------------------------------------|
| Comment      | No       | Lines starting with `#`, describe the test     |
| Directives   | No       | `@skip`, `@group`, `@name`, `@depends`         |
| Command      | Yes      | Shell command executed via `sh -c`             |
| EXIT         | No       | Expected exit code (defaults to `0`)           |
| Body         | No       | Exact stdout match                             |
| [Asserts]    | No       | Rich assertions against stdout/stderr          |

---

## Full Specification

See [SPEC.md](SPEC.md) for the complete syntax reference.

---

## License

MIT
