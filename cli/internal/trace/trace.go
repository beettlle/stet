// Package trace provides a small Tracer for writing internal step output to stderr
// when --trace is set. No-op when the writer is nil.
package trace

import (
	"fmt"
	"io"
)

// Tracer writes sectioned trace output. When the underlying writer is nil, all methods no-op.
type Tracer struct {
	w io.Writer
}

// New returns a Tracer that writes to w. If w is nil, all methods no-op.
func New(w io.Writer) *Tracer {
	return &Tracer{w: w}
}

// Enabled returns true if the tracer has a non-nil writer.
func (t *Tracer) Enabled() bool {
	return t != nil && t.w != nil
}

// Section writes a section header: "\n[stet:trace] === name ===\n"
func (t *Tracer) Section(name string) {
	if !t.Enabled() {
		return
	}
	fmt.Fprintf(t.w, "\n[stet:trace] === %s ===\n", name)
}

// Printf writes to the trace writer when enabled. Format and args are as in fmt.Printf.
func (t *Tracer) Printf(format string, args ...interface{}) {
	if !t.Enabled() {
		return
	}
	fmt.Fprintf(t.w, format, args...)
}
