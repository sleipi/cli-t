package main

import (
	"testing"
	"time"
)

func TestReportSuites_MapsOutcomes(t *testing.T) {
	results := []fileResult{
		{display: "a.clitest", duration: time.Second, outcomes: []entryOutcome{
			{Name: "passes", Passed: true, DurationMs: 5, Stdout: "ignored"},
			{Name: "fails", Failures: []string{"exit: expected 0, got 1"}, Stdout: "out"},
			{Name: "skipped", Skipped: true, SkipReason: "later"},
			{Name: "cleanup", Kind: outcomeDefer, Passed: true, Skipped: true},
		}},
		{display: "hidden.clitest", hidden: true},
		{},
		{display: "broken.clitest", parseErr: "parsing broken.clitest: boom"},
	}

	suites := reportSuites(results)

	if len(suites) != 2 {
		t.Fatalf("expected 2 suites, got %d", len(suites))
	}
	a := suites[0]
	if a.Name != "a.clitest" || a.Time != time.Second || len(a.Cases) != 3 {
		t.Fatalf("unexpected suite: %+v", a)
	}
	if a.Cases[0].Time != 5*time.Millisecond || a.Cases[0].Stdout != "" || len(a.Cases[0].Failures) != 0 {
		t.Errorf("unexpected passing case: %+v", a.Cases[0])
	}
	if a.Cases[1].Failures[0] != "exit: expected 0, got 1" || a.Cases[1].Stdout != "out" {
		t.Errorf("unexpected failing case: %+v", a.Cases[1])
	}
	if !a.Cases[2].Skipped || a.Cases[2].SkipMessage != "later" {
		t.Errorf("unexpected skipped case: %+v", a.Cases[2])
	}
	if suites[1].Error != "parsing broken.clitest: boom" {
		t.Errorf("expected parse error suite, got %+v", suites[1])
	}
}

func TestReportSuites_FailureWithoutMessage(t *testing.T) {
	suites := reportSuites([]fileResult{{display: "a.clitest", outcomes: []entryOutcome{{Name: "x"}}}})

	if got := suites[0].Cases[0].Failures; len(got) != 1 || got[0] != "entry failed" {
		t.Errorf("expected fallback failure, got %v", got)
	}
}
