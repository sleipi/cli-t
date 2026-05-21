package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
)

var sigtestBin string

func TestMain(m *testing.M) {
	// Build sigtest helper once for all tests
	tmp, err := os.MkdirTemp("", "sigtest")
	if err != nil {
		panic(err)
	}

	sigtestBin = filepath.Join(tmp, "sigtest")
	cmd := exec.Command("go", "build", "-o", sigtestBin, "github.com/sleipi/cli-t/test/_helpers/sigtest")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build sigtest: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

func waitForReady(t *testing.T, bp *runner.BackgroundProcess) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(bp.Stdout(), "ready") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("process did not become ready in time")
}

func TestExecuteFinally_SignalAndExit(t *testing.T) {
	bp, err := runner.RunBackground(sigtestBin + " --exit 0")
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	waitForReady(t, bp)

	bgs := []*BackgroundResult{{
		Entry:   types.Entry{Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 3000}},
		Process: bp,
		Command: "test-term",
	}}

	results := ExecuteFinally(bgs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Pass {
		t.Fatalf("expected pass, got: %v", results[0].Failures)
	}
}

func TestExecuteFinally_ExitCodeMismatch(t *testing.T) {
	bp, err := runner.RunBackground(sigtestBin + " --exit 1")
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	waitForReady(t, bp)

	bgs := []*BackgroundResult{{
		Entry:   types.Entry{Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 3000}},
		Process: bp,
		Command: "test-mismatch",
	}}

	results := ExecuteFinally(bgs)
	if results[0].Pass {
		t.Fatal("expected failure for exit code mismatch")
	}
	found := false
	for _, f := range results[0].Failures {
		if f == "[Finally] exit code: expected 0, got 1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected exit code mismatch message, got: %v", results[0].Failures)
	}
}

func TestExecuteFinally_Timeout(t *testing.T) {
	bp, err := runner.RunBackground(sigtestBin + " --ignore")
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	waitForReady(t, bp)

	bgs := []*BackgroundResult{{
		Entry:   types.Entry{Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 200}},
		Process: bp,
		Command: "test-timeout",
	}}

	results := ExecuteFinally(bgs)
	if results[0].Pass {
		t.Fatal("expected failure on timeout")
	}
	_ = bp.Kill()
}

func TestExecuteFinally_PostSignalAsserts(t *testing.T) {
	bp, err := runner.RunBackground(sigtestBin + ` --exit 0 --stderr "shutdown"`)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	waitForReady(t, bp)

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Finally: &types.Finally{
				Signal:   "TERM",
				ExitCode: 0,
				Timeout:  3000,
				Asserts:  []types.Assert{{Query: "stderr", Predicate: "contains", Value: "shutdown"}},
			},
		},
		Process: bp,
		Command: "test-asserts",
	}}

	results := ExecuteFinally(bgs)
	if !results[0].Pass {
		t.Fatalf("expected pass, got: %v", results[0].Failures)
	}
}

func TestExecuteFinally_PostSignalAssertsFail(t *testing.T) {
	bp, err := runner.RunBackground(sigtestBin + " --exit 0")
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	waitForReady(t, bp)

	bgs := []*BackgroundResult{{
		Entry: types.Entry{
			Finally: &types.Finally{
				Signal:   "TERM",
				ExitCode: 0,
				Timeout:  3000,
				Asserts:  []types.Assert{{Query: "stderr", Predicate: "contains", Value: "expected_output"}},
			},
		},
		Process: bp,
		Command: "test-assert-fail",
	}}

	results := ExecuteFinally(bgs)
	if results[0].Pass {
		t.Fatal("expected failure for unmet post-signal assert")
	}
}

func TestExecuteFinally_LIFO(t *testing.T) {
	bp1, err := runner.RunBackground(sigtestBin + " --exit 0")
	if err != nil {
		t.Fatalf("failed to start bp1: %v", err)
	}
	bp2, err := runner.RunBackground(sigtestBin + " --exit 0")
	if err != nil {
		t.Fatalf("failed to start bp2: %v", err)
	}
	waitForReady(t, bp1)
	waitForReady(t, bp2)

	bgs := []*BackgroundResult{
		{
			Entry:   types.Entry{Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 3000}},
			Process: bp1,
			Command: "first",
		},
		{
			Entry:   types.Entry{Finally: &types.Finally{Signal: "TERM", ExitCode: 0, Timeout: 3000}},
			Process: bp2,
			Command: "second",
		},
	}

	results := ExecuteFinally(bgs)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Command != "second" {
		t.Fatalf("expected LIFO order, first result should be 'second', got %q", results[0].Command)
	}
	if results[1].Command != "first" {
		t.Fatalf("expected LIFO order, second result should be 'first', got %q", results[1].Command)
	}
}

func TestExecuteFinally_SkipsEntriesWithoutFinally(t *testing.T) {
	bp, err := runner.RunBackground(sigtestBin + " --exit 0")
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	waitForReady(t, bp)

	bgs := []*BackgroundResult{{
		Entry:   types.Entry{Finally: nil},
		Process: bp,
		Command: "no-finally",
	}}

	results := ExecuteFinally(bgs)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for entry without Finally, got %d", len(results))
	}
	_ = bp.Kill()
}
