package main

import (
	"testing"

	"github.com/sleipi/cli-t/internal/types"
)

func TestFilterEntries_NoFilters(t *testing.T) {
	f := &types.File{
		Entries: []types.Entry{{Command: "a"}, {Command: "b"}},
	}
	got := filterEntries(f, nil, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestFilterEntries_GroupInclude(t *testing.T) {
	f := &types.File{
		Entries: []types.Entry{
			{Command: "a", Directives: types.EntryDirectives{Groups: []string{"fast"}}},
			{Command: "b", Directives: types.EntryDirectives{Groups: []string{"slow"}}},
		},
	}
	got := filterEntries(f, []string{"fast"}, nil)
	if len(got) != 1 || got[0].Command != "a" {
		t.Errorf("expected only entry 'a', got %v", got)
	}
}

func TestFilterEntries_GroupExclude(t *testing.T) {
	f := &types.File{
		Entries: []types.Entry{
			{Command: "a", Directives: types.EntryDirectives{Groups: []string{"fast"}}},
			{Command: "b", Directives: types.EntryDirectives{Groups: []string{"slow"}}},
		},
	}
	got := filterEntries(f, nil, []string{"slow"})
	if len(got) != 1 || got[0].Command != "a" {
		t.Errorf("expected only entry 'a', got %v", got)
	}
}

func TestFilterEntries_BothFilters(t *testing.T) {
	f := &types.File{
		Entries: []types.Entry{
			{Command: "a", Directives: types.EntryDirectives{Groups: []string{"fast", "network"}}},
			{Command: "b", Directives: types.EntryDirectives{Groups: []string{"fast"}}},
			{Command: "c", Directives: types.EntryDirectives{Groups: []string{"slow"}}},
		},
	}
	got := filterEntries(f, []string{"fast"}, []string{"network"})
	if len(got) != 1 || got[0].Command != "b" {
		t.Errorf("expected only entry 'b', got %v", got)
	}
}

func TestFilterEntries_InheritsFileGroups(t *testing.T) {
	f := &types.File{
		Directives: types.FileDirectives{Groups: []string{"integration"}},
		Entries:    []types.Entry{{Command: "a"}},
	}
	got := filterEntries(f, []string{"integration"}, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
}

func TestMergeGroups_BothEmpty(t *testing.T) {
	got := mergeGroups(nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestMergeGroups_FileOnly(t *testing.T) {
	got := mergeGroups([]string{"a"}, nil)
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("expected [a], got %v", got)
	}
}

func TestMergeGroups_EntryOnly(t *testing.T) {
	got := mergeGroups(nil, []string{"b"})
	if len(got) != 1 || got[0] != "b" {
		t.Errorf("expected [b], got %v", got)
	}
}

func TestMergeGroups_Both(t *testing.T) {
	got := mergeGroups([]string{"a"}, []string{"b"})
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestHasAnyTag_Match(t *testing.T) {
	if !hasAnyTag([]string{"a", "b"}, []string{"b"}) {
		t.Error("expected true")
	}
}

func TestHasAnyTag_NoMatch(t *testing.T) {
	if hasAnyTag([]string{"a", "b"}, []string{"c"}) {
		t.Error("expected false")
	}
}
