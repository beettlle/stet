package findings

import (
	"errors"
	"strings"
	"testing"
)

func TestShortID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"empty", "", ""},
		{"short_4", "a1b2", "a1b2"},
		{"short_7", "a1b2c3d", "a1b2c3d"},
		{"long_64", "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456", "a1b2c3d"},
		{"long_8", "deadbeef", "deadbee"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ShortID(tt.id)
			if got != tt.want {
				t.Errorf("ShortID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestResolveFindingIDByPrefix(t *testing.T) {
	t.Parallel()
	full1 := "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456"
	full2 := "a1b2c3d4ffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	full3 := "deadbeef00000000000000000000000000000000000000000000000000000000"
	list := []Finding{
		{ID: full1, File: "a.go", Line: 1, Severity: SeverityInfo, Category: CategoryBug, Confidence: 1.0, Message: "m1"},
		{ID: full2, File: "b.go", Line: 2, Severity: SeverityWarning, Category: CategoryStyle, Confidence: 1.0, Message: "m2"},
		{ID: full3, File: "c.go", Line: 3, Severity: SeverityError, Category: CategorySecurity, Confidence: 1.0, Message: "m3"},
	}

	t.Run("one_match_short_prefix", func(t *testing.T) {
		t.Parallel()
		got, err := ResolveFindingIDByPrefix(list, "dead")
		if err != nil {
			t.Fatalf("ResolveFindingIDByPrefix(list, \"dead\") = _, %v", err)
		}
		if got != full3 {
			t.Errorf("got fullID %q, want %q", got, full3)
		}
	})
	t.Run("one_match_full_id", func(t *testing.T) {
		t.Parallel()
		got, err := ResolveFindingIDByPrefix(list, full1)
		if err != nil {
			t.Fatalf("ResolveFindingIDByPrefix(list, full1) = _, %v", err)
		}
		if got != full1 {
			t.Errorf("got fullID %q, want %q", got, full1)
		}
	})
	t.Run("one_match_seven_chars", func(t *testing.T) {
		t.Parallel()
		// full3 has unique 7-char prefix "deadbee"
		got, err := ResolveFindingIDByPrefix(list, full3[:7])
		if err != nil {
			t.Fatalf("ResolveFindingIDByPrefix(list, 7-char) = _, %v", err)
		}
		if got != full3 {
			t.Errorf("got fullID %q, want %q", got, full3)
		}
	})
	t.Run("case_insensitive", func(t *testing.T) {
		t.Parallel()
		got, err := ResolveFindingIDByPrefix(list, "DEADBEE")
		if err != nil {
			t.Fatalf("ResolveFindingIDByPrefix(list, \"DEADBEE\") = _, %v", err)
		}
		if got != full3 {
			t.Errorf("got fullID %q, want %q", got, full3)
		}
	})
	t.Run("ambiguous", func(t *testing.T) {
		t.Parallel()
		// full1 and full2 share prefix "a1b2c3d"
		_, err := ResolveFindingIDByPrefix(list, "a1b2c3d")
		if err == nil {
			t.Fatal("expected error for ambiguous prefix")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("error = %q, want containing \"ambiguous\"", err.Error())
		}
	})
	t.Run("no_match", func(t *testing.T) {
		t.Parallel()
		_, err := ResolveFindingIDByPrefix(list, "0000000")
		if err == nil {
			t.Fatal("expected error for no match")
		}
		if !strings.Contains(err.Error(), "no finding") {
			t.Errorf("error = %q, want containing \"no finding\"", err.Error())
		}
	})
	t.Run("empty_prefix", func(t *testing.T) {
		t.Parallel()
		_, err := ResolveFindingIDByPrefix(list, "")
		if err == nil {
			t.Fatal("expected error for empty prefix")
		}
		if !strings.Contains(err.Error(), "required") {
			t.Errorf("error = %q, want containing \"required\"", err.Error())
		}
	})
	t.Run("too_short", func(t *testing.T) {
		t.Parallel()
		_, err := ResolveFindingIDByPrefix(list, "abc")
		if err == nil {
			t.Fatal("expected error for prefix length < 4")
		}
		if !errors.Is(err, ErrFindingIDTooShort) {
			t.Errorf("error = %v, want ErrFindingIDTooShort", err)
		}
	})
	t.Run("empty_list", func(t *testing.T) {
		t.Parallel()
		_, err := ResolveFindingIDByPrefix(nil, "a1b2")
		if err == nil {
			t.Fatal("expected error for empty list")
		}
		if !strings.Contains(err.Error(), "no finding") {
			t.Errorf("error = %q, want containing \"no finding\"", err.Error())
		}
	})
	t.Run("skips_empty_id", func(t *testing.T) {
		t.Parallel()
		listWithEmpty := []Finding{
			{ID: "", File: "x.go", Line: 1, Severity: SeverityInfo, Category: CategoryBug, Confidence: 1.0, Message: "x"},
			{ID: full3, File: "c.go", Line: 3, Severity: SeverityError, Category: CategorySecurity, Confidence: 1.0, Message: "m3"},
		}
		got, err := ResolveFindingIDByPrefix(listWithEmpty, "dead")
		if err != nil {
			t.Fatalf("ResolveFindingIDByPrefix = _, %v", err)
		}
		if got != full3 {
			t.Errorf("got fullID %q, want %q", got, full3)
		}
	})
}
