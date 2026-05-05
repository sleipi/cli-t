package main

import (
	"os"
	"strings"

	"github.com/sleipi/clit/internal/runner"
)

func substituteVars(input string, vars map[string]string) string {
	result := input
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	result = os.ExpandEnv(result)
	return result
}

// substituteCaptureVars substitutes only capture variables ({{name}}) without
// expanding environment variables again (those are already expanded on the raw file content).
func substituteCaptureVars(input string, captures map[string]string) string {
	result := input
	for k, v := range captures {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}

func resolveCapture(query string, r runner.Result) string {
	switch query {
	case "stdout":
		return strings.TrimSuffix(r.Stdout, "\n")
	case "stderr":
		return strings.TrimSuffix(r.Stderr, "\n")
	default:
		if strings.HasPrefix(query, "line ") {
			return strings.TrimSuffix(r.Stdout, "\n")
		}
		return ""
	}
}
