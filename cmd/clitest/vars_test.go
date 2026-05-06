package main

import (
	"os"
	"testing"

	"github.com/sleipi/cli-t/internal/runner"
)

func TestSubstituteVars(t *testing.T) {
	vars := map[string]string{"NAME": "world", "VER": "1.0"}
	got := substituteVars("hello {{NAME}} v{{VER}}", vars)
	if got != "hello world v1.0" {
		t.Errorf("expected 'hello world v1.0', got %q", got)
	}
}

func TestSubstituteVars_EnvExpansion(t *testing.T) {
	os.Setenv("CLIT_TEST_VAR", "envval")
	defer os.Unsetenv("CLIT_TEST_VAR")

	got := substituteVars("val=$CLIT_TEST_VAR", nil)
	if got != "val=envval" {
		t.Errorf("expected 'val=envval', got %q", got)
	}
}

func TestSubstituteCaptureVars(t *testing.T) {
	captures := map[string]string{"out": "captured"}
	got := substituteCaptureVars("echo {{out}}", captures)
	if got != "echo captured" {
		t.Errorf("expected 'echo captured', got %q", got)
	}
}

func TestSubstituteCaptureVars_NoEnvExpansion(t *testing.T) {
	os.Setenv("CLIT_TEST_VAR2", "shouldnotappear")
	defer os.Unsetenv("CLIT_TEST_VAR2")

	got := substituteCaptureVars("$CLIT_TEST_VAR2", map[string]string{})
	if got != "$CLIT_TEST_VAR2" {
		t.Errorf("expected literal '$CLIT_TEST_VAR2', got %q", got)
	}
}

func TestResolveCapture_Stdout(t *testing.T) {
	r := runner.Result{Stdout: "hello\n"}
	got := resolveCapture("stdout", r)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestResolveCapture_Stderr(t *testing.T) {
	r := runner.Result{Stderr: "err\n"}
	got := resolveCapture("stderr", r)
	if got != "err" {
		t.Errorf("expected 'err', got %q", got)
	}
}

func TestResolveCapture_Unknown(t *testing.T) {
	r := runner.Result{Stdout: "data"}
	got := resolveCapture("unknown", r)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestResolveCapture_Pid(t *testing.T) {
	r := runner.Result{Pid: 12345}
	got := resolveCapture("pid", r)
	if got != "12345" {
		t.Errorf("expected '12345', got %q", got)
	}
}
