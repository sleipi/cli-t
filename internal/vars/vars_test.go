package vars

import (
	"os"
	"testing"

	"github.com/sleipi/cli-t/internal/runner"
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
	got := ResolveCapture("stdout", r)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestResolveCapture_Stderr(t *testing.T) {
	r := runner.Result{Stderr: "err\n"}
	got := ResolveCapture("stderr", r)
	if got != "err" {
		t.Errorf("expected 'err', got %q", got)
	}
}

func TestResolveCapture_Pid(t *testing.T) {
	r := runner.Result{Pid: 12345}
	got := ResolveCapture("pid", r)
	if got != "12345" {
		t.Errorf("expected '12345', got %q", got)
	}
}
