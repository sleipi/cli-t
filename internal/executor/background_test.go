package executor

import (
	"testing"

	"github.com/sleipi/cli-t/internal/types"
)

func TestBackgroundEntry_PassingAsserts(t *testing.T) {
	entry := types.Entry{
		Command:   `sh -c 'echo ready; sleep 10'`,
		ExitNever: true,
		Asserts:   []types.Assert{{Query: "stdout", Predicate: "contains", Value: "ready"}},
		Directives: types.EntryDirectives{
			Timeout: 3000,
			Poll:    50,
		},
	}
	captures := map[string]string{}
	res, bg := BackgroundEntry(entry, captures)
	if !res.Pass {
		t.Fatalf("expected pass, got failures: %v", res.Failures)
	}
	// No later asserts or finally → process not kept alive
	if bg != nil {
		t.Fatal("expected no BackgroundResult when no later/finally")
		_ = bg.Process.Kill()
	}
}

func TestBackgroundEntry_KeepAliveWithLater(t *testing.T) {
	entry := types.Entry{
		Command:   `sh -c 'echo ready; sleep 10'`,
		ExitNever: true,
		Asserts: []types.Assert{
			{Query: "stdout", Predicate: "contains", Value: "ready"},
			{Query: "stdout", Predicate: "contains", Value: "later_output", Later: true},
		},
		Directives: types.EntryDirectives{
			Timeout: 3000,
			Poll:    50,
		},
	}
	captures := map[string]string{}
	res, bg := BackgroundEntry(entry, captures)
	if !res.Pass {
		t.Fatalf("expected pass, got failures: %v", res.Failures)
	}
	if bg == nil {
		t.Fatal("expected BackgroundResult when later asserts present")
	}
	_ = bg.Process.Kill()
}

func TestBackgroundEntry_KeepAliveWithFinally(t *testing.T) {
	entry := types.Entry{
		Command:   `sh -c 'echo ready; sleep 10'`,
		ExitNever: true,
		Asserts:   []types.Assert{{Query: "stdout", Predicate: "contains", Value: "ready"}},
		Finally:   &types.Finally{Signal: "KILL", ExitCode: 137, Timeout: 1000},
		Directives: types.EntryDirectives{
			Timeout: 3000,
			Poll:    50,
		},
	}
	captures := map[string]string{}
	res, bg := BackgroundEntry(entry, captures)
	if !res.Pass {
		t.Fatalf("expected pass, got failures: %v", res.Failures)
	}
	if bg == nil {
		t.Fatal("expected BackgroundResult when Finally present")
	}
	_ = bg.Process.Kill()
}

func TestBackgroundEntry_Timeout(t *testing.T) {
	entry := types.Entry{
		Command:   `sh -c 'sleep 10'`,
		ExitNever: true,
		Asserts:   []types.Assert{{Query: "stdout", Predicate: "contains", Value: "never_appears"}},
		Directives: types.EntryDirectives{
			Timeout: 200,
			Poll:    50,
		},
	}
	captures := map[string]string{}
	res, bg := BackgroundEntry(entry, captures)
	if res.Pass {
		t.Fatal("expected failure on timeout")
	}
	if bg != nil {
		_ = bg.Process.Kill()
		t.Fatal("expected no BackgroundResult on timeout")
	}
	found := false
	for _, f := range res.Failures {
		if f == "timeout waiting for assertions to pass" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected timeout failure message, got: %v", res.Failures)
	}
}

func TestBackgroundEntry_UnexpectedExit(t *testing.T) {
	entry := types.Entry{
		Command:   `sh -c 'exit 0'`,
		ExitNever: true,
		Asserts:   []types.Assert{{Query: "stdout", Predicate: "contains", Value: "ready"}},
		Directives: types.EntryDirectives{
			Timeout: 3000,
			Poll:    50,
		},
	}
	captures := map[string]string{}
	res, bg := BackgroundEntry(entry, captures)
	if res.Pass {
		t.Fatal("expected failure on unexpected exit")
	}
	if bg != nil {
		t.Fatal("expected no BackgroundResult on unexpected exit")
	}
	found := false
	for _, f := range res.Failures {
		if f == "background process exited unexpectedly" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unexpected exit message, got: %v", res.Failures)
	}
}

func TestBackgroundEntry_CapturesStored(t *testing.T) {
	entry := types.Entry{
		Command:   `sh -c 'echo ready; sleep 10'`,
		ExitNever: true,
		Asserts:   []types.Assert{{Query: "stdout", Predicate: "contains", Value: "ready"}},
		Captures:  []types.Capture{{Name: "bgpid", Query: "pid"}},
		Directives: types.EntryDirectives{
			Timeout: 3000,
			Poll:    50,
		},
	}
	captures := map[string]string{}
	res, _ := BackgroundEntry(entry, captures)
	if !res.Pass {
		t.Fatalf("expected pass, got: %v", res.Failures)
	}
	if captures["bgpid"] == "" || captures["bgpid"] == "0" {
		t.Fatalf("expected pid capture, got %q", captures["bgpid"])
	}
}
