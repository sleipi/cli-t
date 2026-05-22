package runner

import "strings"

// isSimpleCommand returns true if the command does not use shell operators
// that require forking (pipes, redirects, command substitution, etc.).
// Simple commands can safely be prefixed with "exec" to ensure the shell
// replaces itself with the target binary — important for signal delivery
// on shells like dash that don't auto-exec single commands.
func isSimpleCommand(cmd string) bool {
	const meta = "|&;<>(){}$`"
	return !strings.ContainsAny(cmd, meta)
}

// maybeExec inserts "exec" into a simple command so that the shell replaces
// itself with the target binary. Handles ENV=val prefixes correctly by
// inserting exec after all variable assignments.
// Example: "ENV=val ./serve" → "ENV=val exec ./serve"
func maybeExec(cmd string) string {
	if !isSimpleCommand(cmd) {
		return cmd
	}
	trimmed := strings.TrimSpace(cmd)
	if strings.HasPrefix(trimmed, "exec ") {
		return cmd
	}

	// Split into words; find first word that is not a VAR=val assignment
	words := strings.Fields(trimmed)
	execIdx := 0
	for i, w := range words {
		if strings.Contains(w, "=") && !strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "/") && !strings.HasPrefix(w, ".") {
			execIdx = i + 1
		} else {
			break
		}
	}

	if execIdx == 0 {
		return "exec " + cmd
	}

	// Insert "exec" after env assignments
	parts := words[:execIdx]
	rest := words[execIdx:]
	return strings.Join(parts, " ") + " exec " + strings.Join(rest, " ")
}
