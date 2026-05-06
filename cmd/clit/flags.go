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
