package main

import (
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()
	if got := Run(); got != 0 {
		t.Errorf("Run() = %d, want 0", got)
	}
}

func TestRunCLI(t *testing.T) {
	t.Parallel()
	if got := runCLI(nil); got != 0 {
		t.Errorf("runCLI(nil) = %d, want 0", got)
	}
	if got := runCLI([]string{"--help"}); got != 0 {
		t.Errorf("runCLI(--help) = %d, want 0", got)
	}
}

func TestParseArgs(t *testing.T) {
	t.Parallel()
	parseArgs(nil)
	parseArgs([]string{"a", "b"})
}
