package commitmsg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"unicode/utf8"

	"stet/cli/internal/ollama"
)

func TestTruncateUTF8_asciiOnly(t *testing.T) {
	s := "hello world"
	got := truncateUTF8(s, 5)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTruncateUTF8_noTruncationNeeded(t *testing.T) {
	s := "short"
	got := truncateUTF8(s, 100)
	if got != s {
		t.Errorf("got %q, want %q", got, s)
	}
}

func TestTruncateUTF8_exactLength(t *testing.T) {
	s := "exact"
	got := truncateUTF8(s, len(s))
	if got != s {
		t.Errorf("got %q, want %q", got, s)
	}
}

func TestTruncateUTF8_midMultibyte(t *testing.T) {
	// U+00E9 (é) is 2 bytes in UTF-8: 0xC3 0xA9.
	// "café" is 5 bytes: c(1) a(1) f(1) é(2).
	s := "café"
	if len(s) != 5 {
		t.Fatalf("expected 5 bytes, got %d", len(s))
	}

	// Cutting at byte 4 lands on the second byte of é (continuation byte).
	// truncateUTF8 should back up to byte 3, excluding the partial rune.
	got := truncateUTF8(s, 4)
	if got != "caf" {
		t.Errorf("got %q, want %q", got, "caf")
	}
	if !utf8.ValidString(got) {
		t.Errorf("result is not valid UTF-8: %q", got)
	}
}

func TestTruncateUTF8_cutBeforeMultibyte(t *testing.T) {
	// Cutting at byte 3 lands on the first byte of é, which is a RuneStart.
	// The result should include bytes 0..2 (caf minus é).
	s := "café"
	got := truncateUTF8(s, 3)
	if got != "caf" {
		t.Errorf("got %q, want %q", got, "caf")
	}
}

func TestTruncateUTF8_threeByteRune(t *testing.T) {
	// U+4E16 (世) is 3 bytes in UTF-8.
	// "a世b" = a(1) + 世(3) + b(1) = 5 bytes.
	s := "a世b"
	if len(s) != 5 {
		t.Fatalf("expected 5 bytes, got %d", len(s))
	}

	// Cutting at byte 2 or 3 should back up to exclude the partial 世.
	for _, limit := range []int{2, 3} {
		got := truncateUTF8(s, limit)
		if got != "a" {
			t.Errorf("limit=%d: got %q, want %q", limit, got, "a")
		}
		if !utf8.ValidString(got) {
			t.Errorf("limit=%d: result not valid UTF-8: %q", limit, got)
		}
	}

	// Cutting at byte 4 should include the full 世.
	got := truncateUTF8(s, 4)
	if got != "a世" {
		t.Errorf("got %q, want %q", got, "a世")
	}
}

func TestTruncateUTF8_fourByteRune(t *testing.T) {
	// U+1F600 (😀) is 4 bytes in UTF-8.
	s := "x😀y"
	if len(s) != 6 {
		t.Fatalf("expected 6 bytes, got %d", len(s))
	}

	// Cutting at bytes 2, 3, or 4 should back up to exclude partial 😀.
	for _, limit := range []int{2, 3, 4} {
		got := truncateUTF8(s, limit)
		if got != "x" {
			t.Errorf("limit=%d: got %q, want %q", limit, got, "x")
		}
		if !utf8.ValidString(got) {
			t.Errorf("limit=%d: result not valid UTF-8: %q", limit, got)
		}
	}

	// Cutting at byte 5 should include the full 😀.
	got := truncateUTF8(s, 5)
	if got != "x😀" {
		t.Errorf("got %q, want %q", got, "x😀")
	}
}

func TestTruncateUTF8_zeroLimit(t *testing.T) {
	got := truncateUTF8("hello", 0)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestTruncateUTF8_emptyString(t *testing.T) {
	got := truncateUTF8("", 10)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestSuggest_nilClient_returnsError(t *testing.T) {
	ctx := context.Background()
	_, err := Suggest(ctx, nil, "m", "diff", nil)
	if err == nil {
		t.Fatal("Suggest with nil client: want error, got nil")
	}
}

func TestSuggest_longDiffTruncatesAndSucceeds(t *testing.T) {
	// Build a diff longer than maxDiffChars so Suggest truncates it.
	chunk := "line of diff content\n"
	n := (maxDiffChars / len(chunk)) + 2
	longDiff := ""
	for i := 0; i < n; i++ {
		longDiff += chunk
	}
	if len(longDiff) <= maxDiffChars {
		t.Fatalf("test diff len %d <= maxDiffChars %d", len(longDiff), maxDiffChars)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response": "feat: add feature",
			"done":     true,
		})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	ctx := context.Background()
	got, err := Suggest(ctx, client, "m", longDiff, nil)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if got != "feat: add feature" {
		t.Errorf("got %q, want %q", got, "feat: add feature")
	}
}
