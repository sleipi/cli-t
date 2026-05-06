package main

import (
	"testing"
	"time"
)

func TestFormatDuration_Milliseconds(t *testing.T) {
	got := formatDuration(250 * time.Millisecond)
	if got != "250ms" {
		t.Errorf("expected '250ms', got %q", got)
	}
}

func TestFormatDuration_Seconds(t *testing.T) {
	got := formatDuration(2500 * time.Millisecond)
	if got != "2.50s" {
		t.Errorf("expected '2.50s', got %q", got)
	}
}

func TestTruncateCmd_WithinLimit(t *testing.T) {
	got := truncateCmd("short", 10)
	if got != "short" {
		t.Errorf("expected 'short', got %q", got)
	}
}

func TestTruncateCmd_OverLimit(t *testing.T) {
	got := truncateCmd("a very long command string", 10)
	if got != "a very lon..." {
		t.Errorf("expected 'a very lon...', got %q", got)
	}
}
