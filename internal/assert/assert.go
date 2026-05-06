package assert

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
)

// AssertResult holds the outcome of a single assertion.
type AssertResult struct {
	Pass    bool
	Message string
	Assert  types.Assert
}

// Evaluate runs a single assert against a command result.
func Evaluate(a types.Assert, r runner.Result) AssertResult {
	value := resolveQuery(a.Query, r)
	pass := evalPredicate(a.Predicate, value, a.Value)

	if a.Negated {
		pass = !pass
	}

	msg := ""
	if !pass {
		msg = fmt.Sprintf("%s %s%s %s: got %q",
			a.Query, negStr(a.Negated), a.Predicate, quoteVal(a.Value), truncate(value, 100))
	}

	return AssertResult{Pass: pass, Message: msg, Assert: a}
}

// EvaluateBody checks implicit body match.
func EvaluateBody(expectedLines []string, r runner.Result) AssertResult {
	expected := strings.Join(expectedLines, "\n")
	actual := strings.TrimSuffix(r.Stdout, "\n")

	if actual == expected {
		return AssertResult{Pass: true}
	}
	return AssertResult{
		Pass:    false,
		Message: fmt.Sprintf("body mismatch:\n  expected: %q\n  got:      %q", expected, actual),
	}
}

func resolveQuery(query string, r runner.Result) string {
	switch {
	case query == "stdout":
		return strings.TrimSuffix(r.Stdout, "\n")
	case query == "stderr":
		return strings.TrimSuffix(r.Stderr, "\n")
	case query == "lineCount":
		lines := outputLines(r.Stdout)
		return strconv.Itoa(len(lines))
	case query == "duration":
		return strconv.FormatInt(r.DurationMs, 10)
	case query == "exit":
		return strconv.Itoa(r.ExitCode)
	case strings.HasPrefix(query, "line "):
		n, _ := strconv.Atoi(strings.TrimPrefix(query, "line "))
		lines := outputLines(r.Stdout)
		if n >= 1 && n <= len(lines) {
			return lines[n-1]
		}
		return ""
	}
	return ""
}

func outputLines(stdout string) []string {
	s := strings.TrimSuffix(stdout, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func evalPredicate(predicate, actual, expected string) bool {
	switch predicate {
	case "contains":
		return strings.Contains(actual, expected)
	case "startsWith":
		return strings.HasPrefix(actual, expected)
	case "endsWith":
		return strings.HasSuffix(actual, expected)
	case "matches":
		re, err := regexp.Compile(expected)
		if err != nil {
			return false
		}
		return re.MatchString(actual)
	case "isEmpty":
		return actual == ""
	case "==":
		// Try numeric comparison first
		if an, err := strconv.ParseFloat(actual, 64); err == nil {
			if en, err := strconv.ParseFloat(expected, 64); err == nil {
				return an == en
			}
		}
		return actual == expected
	case "!=":
		return actual != expected
	case ">":
		return compareNumeric(actual, expected) > 0
	case ">=":
		return compareNumeric(actual, expected) >= 0
	case "<":
		return compareNumeric(actual, expected) < 0
	case "<=":
		return compareNumeric(actual, expected) <= 0
	}
	return false
}

func compareNumeric(a, b string) int {
	an, _ := strconv.ParseFloat(a, 64)
	bn, _ := strconv.ParseFloat(b, 64)
	if an < bn {
		return -1
	}
	if an > bn {
		return 1
	}
	return 0
}

func negStr(negated bool) string {
	if negated {
		return "not "
	}
	return ""
}

func quoteVal(v string) string {
	if v == "" {
		return ""
	}
	return fmt.Sprintf("%q", v)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
