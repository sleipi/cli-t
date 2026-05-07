package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// resolvedArg holds per-argument resolution info for the header output.
type resolvedArg struct {
	input string
	count int
}

func resolveFiles(args []string, recursive bool) ([]string, []resolvedArg, error) {
	var files []string
	var resolved []resolvedArg
	for _, arg := range args {
		countBefore := len(files)

		var err error
		var skipped bool
		if strings.ContainsAny(arg, "*?[") {
			files, err = resolveGlobArg(arg, recursive, files)
		} else {
			files, skipped, err = resolvePathArg(arg, recursive, files)
		}
		if err != nil {
			return nil, nil, err
		}
		if !skipped {
			resolved = append(resolved, resolvedArg{input: arg, count: len(files) - countBefore})
		}
	}
	sort.Strings(files)
	return files, resolved, nil
}

func resolveGlobArg(pattern string, recursive bool, files []string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			files, err = collectFromDir(m, recursive, files)
			if err != nil {
				return nil, err
			}
		} else {
			if !strings.HasSuffix(m, ".clitest") {
				fmt.Fprintf(os.Stderr, "Warning: skipping non-.clitest file: %s\n", m)
				continue
			}
			files = append(files, m)
		}
	}
	return files, nil
}

func resolvePathArg(arg string, recursive bool, files []string) (result []string, skipped bool, err error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, false, err
	}
	if info.IsDir() {
		files, err = collectFromDir(arg, recursive, files)
		return files, false, err
	}
	if !strings.HasSuffix(arg, ".clitest") {
		fmt.Fprintf(os.Stderr, "Warning: skipping non-.clitest file: %s\n", arg)
		return files, true, nil
	}
	return append(files, arg), false, nil
}

func collectFromDir(dir string, recursive bool, files []string) ([]string, error) {
	if recursive {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".clitest") {
				files = append(files, path)
			}
			return nil
		})
		return files, err
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.clitest"))
	if err != nil {
		return nil, err
	}
	return append(files, matches...), nil
}
