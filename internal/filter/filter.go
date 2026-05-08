package filter

import "github.com/sleipi/cli-t/internal/types"

// Entries returns entries that match group/exclude-group filters.
// File-level groups are inherited by all entries.
func Entries(f *types.File, groups, excludeGroups []string) []types.Entry {
	if len(groups) == 0 && len(excludeGroups) == 0 {
		return f.Entries
	}

	var result []types.Entry
	for _, e := range f.Entries {
		effectiveTags := MergeGroups(f.Directives.Groups, e.Directives.Groups)

		if len(excludeGroups) > 0 && HasAnyTag(effectiveTags, excludeGroups) {
			continue
		}

		if len(groups) > 0 && !HasAnyTag(effectiveTags, groups) {
			continue
		}

		result = append(result, e)
	}
	return result
}

// MergeGroups returns the union of file-level and entry-level groups.
func MergeGroups(fileGroups, entryGroups []string) []string {
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

// HasAnyTag checks if any of the tags is present in effectiveTags (OR logic).
func HasAnyTag(effectiveTags, tags []string) bool {
	for _, t := range tags {
		for _, et := range effectiveTags {
			if et == t {
				return true
			}
		}
	}
	return false
}
