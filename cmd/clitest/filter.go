package main

import "github.com/sleipi/cli-t/internal/types"

// filterEntries returns entries that match group/exclude-group filters.
// File-level groups are inherited by all entries.
func filterEntries(f *types.File, groups []string, excludeGroups []string) []types.Entry {
	if len(groups) == 0 && len(excludeGroups) == 0 {
		return f.Entries
	}

	var result []types.Entry
	for _, e := range f.Entries {
		effectiveTags := mergeGroups(f.Directives.Groups, e.Directives.Groups)

		if len(excludeGroups) > 0 && hasAnyTag(effectiveTags, excludeGroups) {
			continue
		}

		if len(groups) > 0 && !hasAnyTag(effectiveTags, groups) {
			continue
		}

		result = append(result, e)
	}
	return result
}

// mergeGroups returns the union of file-level and entry-level groups.
func mergeGroups(fileGroups, entryGroups []string) []string {
	if len(fileGroups) == 0 {
		return entryGroups
	}
	if len(entryGroups) == 0 {
		return fileGroups
	}
	merged := make([]string, 0, len(fileGroups)+len(entryGroups))
	merged = append(merged, fileGroups...)
	merged = append(merged, entryGroups...)
	return merged
}

// hasAnyTag checks if any of the tags is present in effectiveTags (OR logic).
func hasAnyTag(effectiveTags []string, tags []string) bool {
	for _, t := range tags {
		for _, et := range effectiveTags {
			if et == t {
				return true
			}
		}
	}
	return false
}
