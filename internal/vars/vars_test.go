package vars

import (
	"os"
	"regexp"
	"testing"

	"github.com/sleipi/cli-t/internal/runner"
	"github.com/sleipi/cli-t/internal/types"
)

func TestSubstitute(t *testing.T) {
	v := map[string]string{"NAME": "world", "VER": "1.0"}
	got := Substitute("hello {{NAME}} v{{VER}}", v)
	if got != "hello world v1.0" {
		t.Errorf("expected 'hello world v1.0', got %q", got)
	}
}

func TestSubstitute_EnvExpansion(t *testing.T) {
	os.Setenv("CLIT_TEST_VAR", "envval")
	defer os.Unsetenv("CLIT_TEST_VAR")

	got := Substitute("val=$CLIT_TEST_VAR", nil)
	if got != "val=envval" {
		t.Errorf("expected 'val=envval', got %q", got)
	}
}

func TestSubstituteCaptures(t *testing.T) {
	captures := map[string]string{"out": "captured"}
	got := SubstituteCaptures("echo {{out}}", captures)
	if got != "echo captured" {
		t.Errorf("expected 'echo captured', got %q", got)
	}
}

func TestSubstituteCaptures_NoEnvExpansion(t *testing.T) {
	os.Setenv("CLIT_TEST_VAR2", "shouldnotappear")
	defer os.Unsetenv("CLIT_TEST_VAR2")

	got := SubstituteCaptures("$CLIT_TEST_VAR2", map[string]string{})
	if got != "$CLIT_TEST_VAR2" {
		t.Errorf("expected literal '$CLIT_TEST_VAR2', got %q", got)
	}
}

func TestResolveCapture_Stdout(t *testing.T) {
	r := runner.Result{Stdout: "hello\n"}
	c := types.Capture{Name: "x", Source: types.CaptureStdout}
	got := ResolveCapture(c, r)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestResolveCapture_Stderr(t *testing.T) {
	r := runner.Result{Stderr: "err\n"}
	c := types.Capture{Name: "x", Source: types.CaptureStderr}
	got := ResolveCapture(c, r)
	if got != "err" {
		t.Errorf("expected 'err', got %q", got)
	}
}

func TestResolveCapture_Pid(t *testing.T) {
	r := runner.Result{Pid: 12345}
	c := types.Capture{Name: "x", Source: types.CapturePid}
	got := ResolveCapture(c, r)
	if got != "12345" {
		t.Errorf("expected '12345', got %q", got)
	}
}

func TestResolveCapture_Line(t *testing.T) {
	r := runner.Result{Stdout: "first\nsecond\nthird\n"}
	tests := []struct {
		lineNum int
		want    string
	}{
		{1, "first"},
		{2, "second"},
		{3, "third"},
		{4, ""},  // out of bounds
		{0, ""},  // invalid
	}
	for _, tt := range tests {
		c := types.Capture{Name: "x", Source: types.CaptureLine, LineNum: tt.lineNum}
		got := ResolveCapture(c, r)
		if got != tt.want {
			t.Errorf("line %d: expected %q, got %q", tt.lineNum, tt.want, got)
		}
	}
}

func TestResolveCapture_Line_EmptyStdout(t *testing.T) {
	r := runner.Result{Stdout: ""}
	c := types.Capture{Name: "x", Source: types.CaptureLine, LineNum: 1}
	got := ResolveCapture(c, r)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveCapture_LineCount(t *testing.T) {
	tests := []struct {
		stdout string
		want   string
	}{
		{"", "0"},
		{"one\n", "1"},
		{"one\ntwo\n", "2"},
		{"one\ntwo\nthree\n", "3"},
		{"no trailing newline", "1"},
	}
	for _, tt := range tests {
		r := runner.Result{Stdout: tt.stdout}
		c := types.Capture{Name: "x", Source: types.CaptureLineCount}
		got := ResolveCapture(c, r)
		if got != tt.want {
			t.Errorf("stdout=%q: expected %q, got %q", tt.stdout, tt.want, got)
		}
	}
}

func TestResolveCapture_StdoutRegex_CaptureGroup(t *testing.T) {
	r := runner.Result{Stdout: "version 1.2.3 built 2024\n"}
	re := regexp.MustCompile(`version (\d+\.\d+\.\d+)`)
	c := types.Capture{Name: "ver", Source: types.CaptureStdout, Regex: re}
	got := ResolveCapture(c, r)
	if got != "1.2.3" {
		t.Errorf("expected '1.2.3', got %q", got)
	}
}

func TestResolveCapture_StdoutRegex_FullMatch(t *testing.T) {
	r := runner.Result{Stdout: "abc123def\n"}
	re := regexp.MustCompile(`\d+`)
	c := types.Capture{Name: "num", Source: types.CaptureStdout, Regex: re}
	got := ResolveCapture(c, r)
	if got != "123" {
		t.Errorf("expected '123', got %q", got)
	}
}

func TestResolveCapture_StdoutRegex_NoMatch(t *testing.T) {
	r := runner.Result{Stdout: "no numbers here\n"}
	re := regexp.MustCompile(`\d+`)
	c := types.Capture{Name: "num", Source: types.CaptureStdout, Regex: re}
	got := ResolveCapture(c, r)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveCapture_StderrRegex(t *testing.T) {
	r := runner.Result{Stderr: "error: code=42\n"}
	re := regexp.MustCompile(`code=(\d+)`)
	c := types.Capture{Name: "code", Source: types.CaptureStderr, Regex: re}
	got := ResolveCapture(c, r)
	if got != "42" {
		t.Errorf("expected '42', got %q", got)
	}
}
