package trace

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew_nilWriter_returnsTracer(t *testing.T) {
	tr := New(nil)
	if tr == nil {
		t.Error("New(nil) returned nil")
	}
}

func TestEnabled_nilWriter_returnsFalse(t *testing.T) {
	tr := New(nil)
	if tr.Enabled() {
		t.Error("Enabled() with nil writer = true, want false")
	}
}

func TestEnabled_nonNilWriter_returnsTrue(t *testing.T) {
	var buf bytes.Buffer
	tr := New(&buf)
	if !tr.Enabled() {
		t.Error("Enabled() with non-nil writer = false, want true")
	}
}

func TestSection_nilWriter_noOutput(t *testing.T) {
	tr := New(nil)
	tr.Section("Partition")
	// No panic and no writer to check
}

func TestSection_nonNilWriter_writesHeader(t *testing.T) {
	var buf bytes.Buffer
	tr := New(&buf)
	tr.Section("Partition")
	got := buf.String()
	want := "\n[stet:trace] === Partition ===\n"
	if got != want {
		t.Errorf("Section(%q) wrote %q, want %q", "Partition", got, want)
	}
}

func TestPrintf_nilWriter_noOutput(t *testing.T) {
	tr := New(nil)
	tr.Printf("baseline=%s\n", "HEAD~1")
	// No panic
}

func TestPrintf_nonNilWriter_writesFormatted(t *testing.T) {
	var buf bytes.Buffer
	tr := New(&buf)
	tr.Printf("baseline=%s\n", "HEAD~1")
	got := buf.String()
	want := "baseline=HEAD~1\n"
	if got != want {
		t.Errorf("Printf wrote %q, want %q", got, want)
	}
}

func TestSectionAndPrintf_combined(t *testing.T) {
	var buf bytes.Buffer
	tr := New(&buf)
	tr.Section("Partition")
	tr.Printf("ToReview=%d Approved=%d\n", 3, 0)
	got := buf.String()
	if !strings.Contains(got, "[stet:trace] === Partition ===") {
		t.Errorf("output missing section header: %q", got)
	}
	if !strings.Contains(got, "ToReview=3 Approved=0") {
		t.Errorf("output missing Printf content: %q", got)
	}
}
