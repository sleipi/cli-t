package resolve

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolvedArg holds per-argument resolution info for the header output.
type ResolvedArg struct {
	Input string
	Count int
}

// ScanFile is a resolved .clitest file. Path is used to read the file;
// Display is the name shown to the user, which for files found while
// recursively scanning a directory is the path relative to that directory
// (disambiguating same-named files in different subdirectories).
type ScanFile struct {
	Path    string
	Display string
}

// Files resolves CLI arguments into .clitest file paths.
// It handles individual files, directories (recursively or not), and glob patterns.
// Warnings are returned (not printed) so callers can decide how to display them.
func Files(args []string, recursive bool) (files []ScanFile, resolved []ResolvedArg, warnings []string, err error) {
	for _, arg := range args {
		countBefore := len(files)

		var skipped bool
		if strings.ContainsAny(arg, "*?[") {
			files, warnings, err = resolveGlobArg(arg, recursive, files, warnings)
		} else {
			files, skipped, warnings, err = resolvePathArg(arg, recursive, files, warnings)
		}
		if err != nil {
			return nil, nil, nil, err
		}
		if !skipped {
			resolved = append(resolved, ResolvedArg{Input: arg, Count: len(files) - countBefore})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, resolved, warnings, nil
}

func resolveGlobArg(pattern string, recursive bool, files []ScanFile, warnings []string) (outFiles []ScanFile, outWarnings []string, err error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, warnings, err
	}
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			return nil, warnings, err
		}
		if info.IsDir() {
			files, err = collectFromDir(m, recursive, files)
			if err != nil {
				return nil, warnings, err
			}
		} else {
			if !strings.HasSuffix(m, ".clitest") {
				warnings = append(warnings, fmt.Sprintf("Warning: skipping non-.clitest file: %s", m))
				continue
			}
			files = append(files, ScanFile{Path: m, Display: filepath.Base(m)})
		}
	}
	return files, warnings, nil
}

func resolvePathArg(arg string, recursive bool, files []ScanFile, warnings []string) (result []ScanFile, skipped bool, w []string, err error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, false, warnings, err
	}
	if info.IsDir() {
		files, err = collectFromDir(arg, recursive, files)
		return files, false, warnings, err
	}
	if !strings.HasSuffix(arg, ".clitest") {
		warnings = append(warnings, fmt.Sprintf("Warning: skipping non-.clitest file: %s", arg))
		return files, true, warnings, nil
	}
	return append(files, ScanFile{Path: arg, Display: filepath.Base(arg)}), false, warnings, nil
}

func collectFromDir(dir string, recursive bool, files []ScanFile) ([]ScanFile, error) {
	if recursive {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".clitest") {
				display, relErr := filepath.Rel(dir, path)
				if relErr != nil {
					display = filepath.Base(path)
				}
				files = append(files, ScanFile{Path: path, Display: display})
			}
			return nil
		})
		return files, err
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.clitest"))
	if err != nil {
		return nil, err
	}
	for _, m := range matches {
		files = append(files, ScanFile{Path: m, Display: filepath.Base(m)})
	}
	return files, nil
}
