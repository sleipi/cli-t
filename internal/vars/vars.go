package vars

import (
	"os"
	"strconv"
	"strings"

	"github.com/sleipi/cli-t/internal/runner"
)

// Substitute replaces {{key}} placeholders with values from vars and expands env vars.
func Substitute(input string, vars map[string]string) string {
	result := input
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	result = os.ExpandEnv(result)
	return result
}

// SubstituteCaptures replaces only capture variables ({{name}}) without
// expanding environment variables again.
func SubstituteCaptures(input string, captures map[string]string) string {
	result := input
	for k, v := range captures {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}

// ResolveCapture extracts a value from a runner.Result based on the query string.
func ResolveCapture(query string, r runner.Result) string {
	switch query {
	case "stdout":
		return strings.TrimSuffix(r.Stdout, "\n")
	case "stderr":
		return strings.TrimSuffix(r.Stderr, "\n")
	case "pid":
		return strconv.Itoa(r.Pid)
	default:
		if strings.HasPrefix(query, "line ") {
			return strings.TrimSuffix(r.Stdout, "\n")
		}
		return ""
	}
}
