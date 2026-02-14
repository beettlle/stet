package tokens

import (
	"math"
	"strings"
	"testing"
)

func TestEstimate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
		want   int
	}{
		{"empty", "", 0},
		{"one_char", "x", 1},
		{"two_chars", "ab", 1},
		{"three_chars", "abc", 1},
		{"four_chars", "abcd", 1},
		{"five_chars", "abcde", 2},
		{"eight_chars", "abcdefgh", 2},
		{"twelve_chars", "abcdabcdabcd", 3},
		{"100_chars", strings.Repeat("x", 100), 25},
		{"unicode_multi_byte", "café", 2}, // é is 2 bytes in UTF-8; total 5 bytes → 2 tokens
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Estimate(tt.prompt)
			if got != tt.want {
				t.Errorf("Estimate(%q) = %d, want %d", tt.prompt, got, tt.want)
			}
		})
	}
}

func TestWarnIfOver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		promptTokens    int
		responseReserve int
		contextLimit    int
		warnThreshold   float64
		wantEmpty       bool
		wantContains    []string
	}{
		{"under_threshold", 1000, 500, 32768, 0.9, true, nil},
		{"at_threshold", 29491, DefaultResponseReserve, 32768, 0.9, false, []string{"29491", "90", "32768"}},
		{"over_threshold", 30000, DefaultResponseReserve, 32768, 0.9, false, []string{"32048", "prompt 30000", "reserve 2048"}},
		{"context_limit_zero", 100, 0, 0, 0.9, true, nil},
		{"context_limit_negative", 100, 0, -1, 0.9, true, nil},
		{"warn_threshold_zero_total_positive", 1, 0, 100, 0, false, []string{"exceeds", "0%"}},
		{"warn_threshold_one_at_limit", 32768, 0, 32768, 1.0, false, []string{"32768", "100", "32768"}},
		{"warn_threshold_one_under_limit", 32767, 0, 32768, 1.0, true, nil},
		{"overflow_returns_warning", math.MaxInt, 1, 32768, 0.9, false, []string{"overflow", "prompt", "reserve"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := WarnIfOver(tt.promptTokens, tt.responseReserve, tt.contextLimit, tt.warnThreshold)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("WarnIfOver(...) = %q, want empty", got)
				}
				return
			}
			if got == "" && len(tt.wantContains) > 0 {
				t.Fatalf("WarnIfOver(...) = %q, want non-empty containing %v", got, tt.wantContains)
			}
			for _, sub := range tt.wantContains {
				if !strings.Contains(got, sub) {
					t.Errorf("WarnIfOver(...) = %q, want to contain %q", got, sub)
				}
			}
		})
	}
}

func TestDefaultResponseReserve_value(t *testing.T) {
	t.Parallel()
	if DefaultResponseReserve <= 0 {
		t.Errorf("DefaultResponseReserve = %d, want positive", DefaultResponseReserve)
	}
}
