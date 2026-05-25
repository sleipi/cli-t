package vars

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
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

// ResolveCapture extracts a value from a runner.Result based on the capture definition.
func ResolveCapture(c types.Capture, r runner.Result) string {
	switch c.Source {
	case types.CaptureStdout:
		s := strings.TrimSuffix(r.Stdout, "\n")
		if c.Regex != nil {
			return regexExtract(c.Regex, s)
		}
		return s
	case types.CaptureStderr:
		s := strings.TrimSuffix(r.Stderr, "\n")
		if c.Regex != nil {
			return regexExtract(c.Regex, s)
		}
		return s
	case types.CapturePid:
		return strconv.Itoa(r.Pid)
	case types.CaptureLine:
		s := strings.TrimSuffix(r.Stdout, "\n")
		if s == "" {
			return ""
		}
		lines := strings.Split(s, "\n")
		if c.LineNum < 1 || c.LineNum > len(lines) {
			return ""
		}
		return lines[c.LineNum-1]
	case types.CaptureLineCount:
		s := strings.TrimSuffix(r.Stdout, "\n")
		if s == "" {
			return "0"
		}
		return strconv.Itoa(len(strings.Split(s, "\n")))
	}
	return ""
}

// regexExtract returns the first capture group match, or the full match if no groups.
func regexExtract(re *regexp.Regexp, text string) string {
	m := re.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	if len(m) > 1 {
		return m[1]
	}
	return m[0]
}
