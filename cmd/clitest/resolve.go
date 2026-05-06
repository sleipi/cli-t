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

		if strings.ContainsAny(arg, "*?[") {
			matches, err := filepath.Glob(arg)
			if err != nil {
				return nil, nil, err
			}
			for _, m := range matches {
				info, err := os.Stat(m)
				if err != nil {
					return nil, nil, err
				}
				if info.IsDir() {
					if recursive {
						err = filepath.WalkDir(m, func(path string, d fs.DirEntry, err error) error {
							if err != nil {
								return err
							}
							if !d.IsDir() && strings.HasSuffix(path, ".clitest") {
								files = append(files, path)
							}
							return nil
						})
						if err != nil {
							return nil, nil, err
						}
					} else {
						dirMatches, err := filepath.Glob(filepath.Join(m, "*.clitest"))
						if err != nil {
							return nil, nil, err
						}
						files = append(files, dirMatches...)
					}
				} else {
					if !strings.HasSuffix(m, ".clitest") {
						fmt.Fprintf(os.Stderr, "Warning: skipping non-.clitest file: %s\n", m)
						continue
					}
					files = append(files, m)
				}
			}
			resolved = append(resolved, resolvedArg{input: arg, count: len(files) - countBefore})
			continue
		}

		info, err := os.Stat(arg)
		if err != nil {
			return nil, nil, err
		}
		if info.IsDir() {
			if recursive {
				err = filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() && strings.HasSuffix(path, ".clitest") {
						files = append(files, path)
					}
					return nil
				})
				if err != nil {
					return nil, nil, err
				}
			} else {
				matches, err := filepath.Glob(filepath.Join(arg, "*.clitest"))
				if err != nil {
					return nil, nil, err
				}
				files = append(files, matches...)
			}
		} else {
			if !strings.HasSuffix(arg, ".clitest") {
				fmt.Fprintf(os.Stderr, "Warning: skipping non-.clitest file: %s\n", arg)
				continue
			}
			files = append(files, arg)
		}
		resolved = append(resolved, resolvedArg{input: arg, count: len(files) - countBefore})
	}
	sort.Strings(files)
	return files, resolved, nil
}
