package main

import (
	"fmt"
	"strings"
)

// varMap implements pflag.Value for repeated --var flags
type varMap struct {
	values map[string]string
}

func (v *varMap) String() string { return "" }
func (v *varMap) Set(s string) error {
	if v.values == nil {
		v.values = make(map[string]string)
	}
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid var format, use NAME=VALUE")
	}
	v.values[parts[0]] = parts[1]
	return nil
}
func (v *varMap) Type() string { return "NAME=VALUE" }

// parseReport splits a --report value of the form FORMAT:PATH.
func parseReport(s string) (format, path string, err error) {
	format, path, found := strings.Cut(s, ":")
	if !found || path == "" {
		return "", "", fmt.Errorf("invalid --report value %q, use junit:PATH", s)
	}
	if format != "junit" {
		return "", "", fmt.Errorf("unsupported report format %q, supported: junit", format)
	}
	return format, path, nil
}
