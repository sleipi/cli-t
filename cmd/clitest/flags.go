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

// reportValue implements pflag.Value for --report FORMAT:PATH. Only one report is allowed.
type reportValue struct {
	path string
}

func (r *reportValue) String() string { return r.path }
func (r *reportValue) Set(s string) error {
	if r.path != "" {
		return fmt.Errorf("--report can only be given once")
	}
	format, path, found := strings.Cut(s, ":")
	if !found || path == "" {
		return fmt.Errorf("invalid value %q, use junit:PATH", s)
	}
	if format != "junit" {
		return fmt.Errorf("unsupported report format %q, supported: junit", format)
	}
	r.path = path
	return nil
}
func (r *reportValue) Type() string { return "FORMAT:PATH" }
