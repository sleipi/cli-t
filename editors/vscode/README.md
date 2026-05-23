# clitest — Syntax Highlighting

TextMate grammar for `.clitest` files. Provides syntax highlighting in VS Code, IntelliJ IDEA, and Sublime Text.

## VS Code

### Install from source

1. Copy (or symlink) the `editors/vscode` directory into your VS Code extensions folder:

   ```bash
   # macOS / Linux
   ln -s /path/to/cli-t/editors/vscode ~/.vscode/extensions/clitest

   # Or copy
   cp -r /path/to/cli-t/editors/vscode ~/.vscode/extensions/clitest
   ```

2. Restart VS Code (or reload window: `Cmd+Shift+P` → "Reload Window")

3. Open any `.clitest` file — syntax highlighting should activate automatically.

## IntelliJ IDEA

IntelliJ supports TextMate bundles natively since 2019.x.

1. Open **Settings** → **Editor** → **TextMate Bundles**
2. Click **+** (Add)
3. Navigate to the `editors/vscode` directory in this repository
4. Click **OK**

All `.clitest` files will now have syntax highlighting.

> Works in all JetBrains IDEs: IntelliJ IDEA, GoLand, WebStorm, PhpStorm, PyCharm, etc.

## Sublime Text

1. Copy `syntaxes/clitest.tmLanguage.json` to your Sublime packages directory:

   ```bash
   # macOS
   cp editors/vscode/syntaxes/clitest.tmLanguage.json \
     ~/Library/Application\ Support/Sublime\ Text/Packages/User/clitest.tmLanguage.json
   ```

2. Restart Sublime Text.

## What gets highlighted

| Element | Example |
|---------|---------|
| Comments | `# Test description` |
| Directives | `@group smoke`, `@timeout 5000`, `@skip reason` |
| EXIT | `EXIT 0`, `EXIT NEVER` |
| Sections | `[Asserts]`, `[Captures]`, `[Prompts]`, `[Finally]` |
| Queries | `stdout`, `stderr`, `exit`, `lineCount`, `duration`, `line 1` |
| Predicates | `contains`, `matches`, `startsWith`, `isEmpty`, `==`, `<` |
| Negation | `not` |
| Strings | `"hello world"` |
| Regex | `/\d+\.\d+/` |
| Variables | `{{name}}` |
| Prompts | `"pattern" => "response" * 3` |
| Frontmatter | `---` delimited block |
| Finally signals | `TERM EXIT 0 timeout 3000` |
