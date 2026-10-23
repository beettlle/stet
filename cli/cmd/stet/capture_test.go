package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

// captureOutput runs fn with os.Stdout and os.Stderr replaced by os.Pipe
// pipes and returns everything written to each stream while fn ran.
//
// It must NOT be used from parallel tests: swapping os.Stdout and os.Stderr
// is process-wide, so a concurrent writer to the real streams would race on
// those variables and have its output stolen into these pipes.
//
// Each read end is drained in a goroutine while fn runs, so fn cannot block
// on a full OS pipe buffer (64 KiB on Linux). On return — including when fn
// panics or calls t.Fatal, because the restore runs via defer — the original
// streams are put back, both write ends are closed so the drains see EOF, the
// drain goroutines are waited on, and both read ends are closed, leaving no
// file descriptors behind.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	origStdout, origStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutW, stderrW

	var stdoutBuf, stderrBuf bytes.Buffer
	var drains sync.WaitGroup
	drains.Add(2)
	go func() {
		defer drains.Done()
		_, _ = io.Copy(&stdoutBuf, stdoutR)
	}()
	go func() {
		defer drains.Done()
		_, _ = io.Copy(&stderrBuf, stderrR)
	}()

	defer func() {
		os.Stdout, os.Stderr = origStdout, origStderr
		// Close the write ends so the drains reach EOF even if fn panicked
		// before writing; wait so the buffers are complete before returning.
		_ = stdoutW.Close()
		_ = stderrW.Close()
		drains.Wait()
		_ = stdoutR.Close()
		_ = stderrR.Close()
		// Assigned here (not at a plain return) so the buffers are read only
		// after both drains have finished copying.
		stdout, stderr = stdoutBuf.String(), stderrBuf.String()
	}()

	fn()
	return
}

func TestCaptureOutput_largeStdoutCompletesWithoutBlocking(t *testing.T) {
	// Regression guard: output larger than the OS pipe buffer (64 KiB on
	// Linux) must not deadlock, because the drain runs while fn writes.
	const size = 160 * 1024 // > 128 KiB
	payload := strings.Repeat("x", size)
	var wrote int
	stdout, stderr := captureOutput(t, func() {
		n, _ := fmt.Fprint(os.Stdout, payload)
		wrote = n
	})
	if wrote != size {
		t.Fatalf("fn wrote %d bytes, want %d", wrote, size)
	}
	if len(stdout) != size {
		t.Errorf("captured %d stdout bytes, want %d", len(stdout), size)
	}
	if stdout != payload {
		t.Error("captured stdout does not match the payload")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestCaptureOutput_separatesStreams(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		fmt.Fprint(os.Stdout, "to-stdout")
		fmt.Fprint(os.Stderr, "to-stderr")
	})
	if stdout != "to-stdout" {
		t.Errorf("stdout = %q, want %q", stdout, "to-stdout")
	}
	if stderr != "to-stderr" {
		t.Errorf("stderr = %q, want %q", stderr, "to-stderr")
	}
}

func TestCaptureOutput_restoresStreams(t *testing.T) {
	origStdout, origStderr := os.Stdout, os.Stderr
	_, _ = captureOutput(t, func() {
		fmt.Fprint(os.Stdout, "x")
		fmt.Fprint(os.Stderr, "y")
	})
	if os.Stdout != origStdout {
		t.Error("os.Stdout was not restored after captureOutput")
	}
	if os.Stderr != origStderr {
		t.Error("os.Stderr was not restored after captureOutput")
	}
}

func TestCaptureOutput_restoresStreamsAfterPanic(t *testing.T) {
	origStdout, origStderr := os.Stdout, os.Stderr
	func() {
		defer func() { _ = recover() }() // fn panics; captureOutput must still restore
		_, _ = captureOutput(t, func() {
			panic("boom")
		})
	}()
	if os.Stdout != origStdout {
		t.Error("os.Stdout was not restored after fn panicked")
	}
	if os.Stderr != origStderr {
		t.Error("os.Stderr was not restored after fn panicked")
	}
}
