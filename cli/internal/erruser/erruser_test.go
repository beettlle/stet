package erruser

import (
	"errors"
	"testing"
)

func TestErr_Error_returnsMsgOnly(t *testing.T) {
	t.Parallel()
	cause := errors.New("exit status 128")
	e := New("This directory is not inside a Git repository.", cause)
	if got := e.Error(); got != "This directory is not inside a Git repository." {
		t.Errorf("Error() = %q, want user message only", got)
	}
	if !errors.Is(e, cause) {
		t.Error("errors.Is(e, cause) should be true")
	}
	var unwrapped *Err
	if !errors.As(e, &unwrapped) {
		t.Fatal("errors.As to *Err failed")
	}
	if unwrapped.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want cause", unwrapped.Unwrap())
	}
}

func TestNew_nilErr_returnsSimpleError(t *testing.T) {
	t.Parallel()
	e := New("Something went wrong.", nil)
	if e.Error() != "Something went wrong." {
		t.Errorf("Error() = %q", e.Error())
	}
	if errors.Unwrap(e) != nil {
		t.Errorf("Unwrap() should be nil for New(msg, nil), got %v", errors.Unwrap(e))
	}
}

func TestErr_nilReceiver_noPanic(t *testing.T) {
	t.Parallel()
	var e *Err
	if got := e.Error(); got != "" {
		t.Errorf("(*Err)(nil).Error() = %q, want %q", got, "")
	}
	if e.Unwrap() != nil {
		t.Errorf("(*Err)(nil).Unwrap() = %v, want nil", e.Unwrap())
	}
}
