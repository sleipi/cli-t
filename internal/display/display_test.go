package display

import (
	"bytes"
	"strings"
	"testing"
)

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		done      bool
		expected  string
	}{
		{"zero progress", 0, 5, false, "[>         ]"},
		{"half progress", 5, 10, false, "[=====>    ]"},
		{"full progress done", 5, 5, true, "[==========]"},
		{"one of three", 1, 3, false, "[===>      ]"},
		{"two of three", 2, 3, false, "[======>   ]"},
		{"single entry running", 0, 1, false, "[>         ]"},
		{"single entry done", 1, 1, true, "[==========]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderBar(tt.completed, tt.total, tt.done)
			if got != tt.expected {
				t.Errorf("RenderBar(%d, %d, %v) = %q, want %q", tt.completed, tt.total, tt.done, got, tt.expected)
			}
		})
	}
}

func TestProgressDisplay_RenderRunning(t *testing.T) {
	var buf bytes.Buffer
	d := NewProgressDisplay(&buf, true, 8)
	d.Start([]string{"01_basic.clitest", "02_errors.clitest"})

	d.UpdateProgress(0, 1, 3)
	d.Finish()

	output := buf.String()
	if !strings.Contains(output, "RUN") {
		t.Errorf("expected RUN in output, got:\n%s", output)
	}
	if !strings.Contains(output, "01_basic.clitest") {
		t.Errorf("expected filename in output, got:\n%s", output)
	}
}

func TestProgressDisplay_RenderComplete(t *testing.T) {
	var buf bytes.Buffer
	d := NewProgressDisplay(&buf, false, 8)
	d.Start([]string{"01_basic.clitest"})
	d.UpdateProgress(0, 0, 3)
	d.UpdateProgress(0, 3, 3)

	d.FinishFile(0, true, "")
	d.Finish()

	output := buf.String()
	if !strings.Contains(output, "OK") {
		t.Errorf("expected OK in output, got:\n%s", output)
	}
	if !strings.Contains(output, "[==========]") {
		t.Errorf("expected full bar in output, got:\n%s", output)
	}
	if !strings.Contains(output, "(3/3)") {
		t.Errorf("expected counter (3/3) in output, got:\n%s", output)
	}
	if !strings.Contains(output, "took") {
		t.Errorf("expected 'took' in output, got:\n%s", output)
	}
}

func TestProgressDisplay_RenderFailed(t *testing.T) {
	var buf bytes.Buffer
	d := NewProgressDisplay(&buf, false, 8)
	d.Start([]string{"02_errors.clitest"})
	d.UpdateProgress(0, 0, 2)
	d.UpdateProgress(0, 2, 2)

	d.FinishFile(0, false, "")
	d.Finish()

	output := buf.String()
	if !strings.Contains(output, "ERR") {
		t.Errorf("expected ERR in output, got:\n%s", output)
	}
	if !strings.Contains(output, "[==========]") {
		t.Errorf("expected full bar in output, got:\n%s", output)
	}
	if !strings.Contains(output, "(2/2)") {
		t.Errorf("expected counter (2/2) in output, got:\n%s", output)
	}
	if !strings.Contains(output, "took") {
		t.Errorf("expected 'took' in output, got:\n%s", output)
	}
}

func TestProgressDisplay_StaticMode_PrintsOnFinish(t *testing.T) {
	var buf bytes.Buffer
	d := NewProgressDisplay(&buf, false, 8)
	d.Start([]string{"a.clitest", "b.clitest"})

	d.UpdateProgress(0, 1, 3)
	d.UpdateProgress(0, 2, 3)
	d.FinishFile(0, true, "")
	d.FinishFile(1, false, "")
	d.Finish()

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), output)
	}
	if !strings.Contains(lines[0], "OK") {
		t.Errorf("first line should be OK, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "ERR") {
		t.Errorf("second line should be ERR, got: %s", lines[1])
	}
}

func TestProgressDisplay_DynamicMode_UpdatesInPlace(t *testing.T) {
	var buf bytes.Buffer
	d := NewProgressDisplay(&buf, true, 8)
	d.Start([]string{"a.clitest", "b.clitest"})

	d.UpdateProgress(0, 1, 3)
	d.FinishFile(0, true, "")
	d.FinishFile(1, true, "")
	d.Finish()

	output := buf.String()
	if !strings.Contains(output, "\033[") {
		t.Errorf("expected ANSI escape sequences in dynamic mode, got:\n%q", output)
	}
	if !strings.Contains(output, "(1/3)") && !strings.Contains(output, "(3/3)") {
		t.Errorf("expected counter in dynamic output, got:\n%q", output)
	}
}

func TestVerboseDisplay_EntryPass(t *testing.T) {
	var buf bytes.Buffer
	d := NewVerboseDisplay(&buf, false)
	d.BeginFile("01_basic.clitest")
	d.EntryResult(EntryInfo{
		Command:     "echo hello",
		Passed:      true,
		ExitCode:    0,
		AssertCount: 1,
	})

	output := buf.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected ✓ in verbose output, got:\n%s", output)
	}
	if !strings.Contains(output, "echo hello") {
		t.Errorf("expected command in output, got:\n%s", output)
	}
}

func TestVerboseDisplay_EntryFail(t *testing.T) {
	var buf bytes.Buffer
	d := NewVerboseDisplay(&buf, false)
	d.BeginFile("01_basic.clitest")
	d.EntryResult(EntryInfo{
		Command:  "cat /nope",
		Passed:   false,
		Failures: []string{"exit code: expected 0, got 1"},
		Stdout:   "",
		Stderr:   "cat: /nope: No such file or directory",
	})

	output := buf.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected ✗ in verbose output, got:\n%s", output)
	}
	if !strings.Contains(output, "FAIL:") {
		t.Errorf("expected FAIL: in output, got:\n%s", output)
	}
}

func TestVerboseDisplay_ShowsFileHeader(t *testing.T) {
	var buf bytes.Buffer
	d := NewVerboseDisplay(&buf, false)
	d.BeginFile("my_test.clitest")

	output := buf.String()
	if !strings.Contains(output, "▶ my_test.clitest") {
		t.Errorf("expected file header in verbose output, got:\n%s", output)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello\n", 1},
		{"a\nb\nc\n", 3},
		{"no newline", 0},
	}
	for _, tt := range tests {
		got := CountLines(tt.input)
		if got != tt.expected {
			t.Errorf("CountLines(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestFormatDuration_Milliseconds(t *testing.T) {
	got := FormatDuration(250 * 1000000) // 250ms
	if got != "250ms" {
		t.Errorf("expected '250ms', got %q", got)
	}
}

func TestFormatDuration_Seconds(t *testing.T) {
	got := FormatDuration(2500 * 1000000) // 2500ms
	if got != "2.50s" {
		t.Errorf("expected '2.50s', got %q", got)
	}
}

func TestTruncateCmd_WithinLimit(t *testing.T) {
	got := TruncateCmd("short", 10)
	if got != "short" {
		t.Errorf("expected 'short', got %q", got)
	}
}

func TestTruncateCmd_OverLimit(t *testing.T) {
	got := TruncateCmd("a very long command string", 10)
	if got != "a very lon..." {
		t.Errorf("expected 'a very lon...', got %q", got)
	}
}

func TestProgressDisplay_HideFile(t *testing.T) {
	var buf bytes.Buffer
	d := NewProgressDisplay(&buf, false, 8)
	d.Start([]string{"a.clitest", "b.clitest"})

	d.HideFile(0)
	d.FinishFile(1, true, "")
	d.Finish()

	output := buf.String()
	if strings.Contains(output, "a.clitest") {
		t.Errorf("hidden file should not appear in output, got:\n%s", output)
	}
	if !strings.Contains(output, "b.clitest") {
		t.Errorf("non-hidden file should appear in output, got:\n%s", output)
	}
}

func TestProgressDisplay_FileError(t *testing.T) {
	var buf bytes.Buffer
	d := NewProgressDisplay(&buf, false, 8)
	d.Start([]string{"bad.clitest"})

	d.FileError(0, "parse error: unexpected token\n")
	d.Finish()

	output := buf.String()
	if !strings.Contains(output, "parse error: unexpected token") {
		t.Errorf("expected error message in output, got:\n%s", output)
	}
}

func TestProgressDisplay_FinishFileWithCustomOutput(t *testing.T) {
	var buf bytes.Buffer
	d := NewProgressDisplay(&buf, false, 8)
	d.Start([]string{"a.clitest"})

	d.FinishFile(0, true, "custom output line\n")
	d.Finish()

	output := buf.String()
	if !strings.Contains(output, "custom output line") {
		t.Errorf("expected custom output in result, got:\n%s", output)
	}
}

func TestVerboseDisplay_DeferResult(t *testing.T) {
	var buf bytes.Buffer
	d := NewVerboseDisplay(&buf, false)
	d.DeferResult("rm -f /tmp/test")

	output := buf.String()
	if !strings.Contains(output, "~") {
		t.Errorf("expected ~ marker for defer, got:\n%s", output)
	}
	if !strings.Contains(output, "[defer]") {
		t.Errorf("expected [defer] label, got:\n%s", output)
	}
	if !strings.Contains(output, "rm -f /tmp/test") {
		t.Errorf("expected command in defer output, got:\n%s", output)
	}
}

func TestVerboseDisplay_SkippedEntry(t *testing.T) {
	var buf bytes.Buffer
	d := NewVerboseDisplay(&buf, false)
	d.EntryResult(EntryInfo{
		Command:    "curl http://localhost",
		Skipped:    true,
		SkipReason: "service unavailable",
	})

	output := buf.String()
	if !strings.Contains(output, "⊘") {
		t.Errorf("expected ⊘ marker for skip, got:\n%s", output)
	}
	if !strings.Contains(output, "SKIP") {
		t.Errorf("expected SKIP label, got:\n%s", output)
	}
	if !strings.Contains(output, "service unavailable") {
		t.Errorf("expected skip reason, got:\n%s", output)
	}
}

func TestVerboseDisplay_VerboseShowsStdoutOnPass(t *testing.T) {
	var buf bytes.Buffer
	d := NewVerboseDisplay(&buf, true) // verbose=true
	d.EntryResult(EntryInfo{
		Command:     "echo hello",
		Passed:      true,
		ExitCode:    0,
		AssertCount: 1,
		Stdout:      "hello\n",
	})

	output := buf.String()
	if !strings.Contains(output, "stdout") {
		t.Errorf("verbose=true should show stdout on pass, got:\n%s", output)
	}
	if !strings.Contains(output, "hello") {
		t.Errorf("expected stdout content, got:\n%s", output)
	}
}

func TestVerboseDisplay_NonVerboseHidesStdoutOnPass(t *testing.T) {
	var buf bytes.Buffer
	d := NewVerboseDisplay(&buf, false) // verbose=false
	d.EntryResult(EntryInfo{
		Command:     "echo hello",
		Passed:      true,
		ExitCode:    0,
		AssertCount: 1,
		Stdout:      "hello\n",
	})

	output := buf.String()
	if strings.Contains(output, "stdout") {
		t.Errorf("verbose=false should NOT show stdout on pass, got:\n%s", output)
	}
}
