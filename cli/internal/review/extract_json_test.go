// Tests for recovering findings JSON embedded in prose responses (SP-040):
// fenced code blocks and balanced JSON spans, newest candidate first.
package review

import (
	"strings"
	"testing"

	"stet/cli/internal/findings"
)

const recoveryFindingJSON = `{"file":"x.py","line":12,"severity":"warning","category":"bug","message":"off by one","confidence":0.8}`

// TestParseFindingsResponse_recovery covers prose-wrapped findings payloads:
// fenced blocks and bare JSON spans, newest valid candidate wins, empty array
// is valid, and unrecoverable prose keeps the original strict-parse error.
func TestParseFindingsResponse_recovery(t *testing.T) {
	tests := []struct {
		name       string
		resp       string
		wantLen    int    // expected finding count when wantErr == ""
		wantErrSub string // when non-empty, parse must fail with error containing this
		wantFile   string // when non-empty, list[0].File must equal this
	}{
		{
			// The observed failure shape: prose analysis, then a fenced json array.
			name: "prose_then_fenced_json_array",
			resp: "Looking at the code, I need to check the loop bounds.\n\nThe variable `i` starts at 1, which skips index 0. Note [brackets] and {braces} in prose.\n\n```json\n[" + recoveryFindingJSON + "]\n```\n",
			wantLen: 1, wantFile: "x.py",
		},
		{
			name: "prose_then_bare_array",
			resp: "Analysis: the diff touches the loop. Inline `code` and [bracket] text.\n\n[" + recoveryFindingJSON + "]\n",
			wantLen: 1, wantFile: "x.py",
		},
		{
			name: "prose_with_junk_brackets_before_real_array",
			resp: "First [x] then {y} plus `inline code` and [a, b] lists.\n\n[" + recoveryFindingJSON + "]\n",
			wantLen: 1, wantFile: "x.py",
		},
		{
			name: "two_fenced_blocks_last_valid_wins",
			resp: "```json\n[" + recoveryFindingJSON + "]\n```\nWait, correcting myself:\n\n```json\n[{\"file\":\"new.py\",\"line\":20,\"severity\":\"info\",\"category\":\"bug\",\"message\":\"newer finding\",\"confidence\":0.7}]\n```",
			wantLen: 1, wantFile: "new.py",
		},
		{
			name: "last_fenced_block_truncated_earlier_valid_wins",
			resp: "```json\n[" + recoveryFindingJSON + "]\n```\nAdditional notes:\n\n```json\n[{\"file\":\"new.py\",\"line\":20",
			wantLen: 1, wantFile: "x.py",
		},
		{
			name: "last_bare_span_truncated_earlier_valid_wins",
			resp: "[" + recoveryFindingJSON + "]\nCorrection attempt: [{\"file\":\"new.py\"",
			wantLen: 1, wantFile: "x.py",
		},
		{
			name: "wrapper_in_prose",
			resp: "Here is my review:\n{\"findings\":[" + recoveryFindingJSON + "]}\nHope that helps!",
			wantLen: 1, wantFile: "x.py",
		},
		{
			name:    "empty_array_in_prose",
			resp:    "No issues found in this hunk.\n\n[]",
			wantLen: 0,
		},
		{
			name:    "fenced_empty_array_in_prose",
			resp:    "I checked the hunk and found nothing.\n\n```json\n[]\n```",
			wantLen: 0,
		},
		{
			name:       "pure_prose_original_error",
			resp:       "Looking at the code, I need to check several things before concluding.",
			wantErrSub: "invalid character 'L'",
		},
		{
			name:    "strict_array_unchanged",
			resp:    "[" + recoveryFindingJSON + "]",
			wantLen: 1, wantFile: "x.py",
		},
		{
			name:    "leading_fence_unchanged",
			resp:    "```json\n[" + recoveryFindingJSON + "]\n```",
			wantLen: 1, wantFile: "x.py",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			list, err := ParseFindingsResponse(tt.resp)
			if tt.wantErrSub != "" {
				if err == nil {
					t.Fatalf("ParseFindingsResponse: want error containing %q, got nil (list=%+v)", tt.wantErrSub, list)
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("ParseFindingsResponse: error %q want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFindingsResponse: %v", err)
			}
			if len(list) != tt.wantLen {
				t.Fatalf("len(list) = %d, want %d", len(list), tt.wantLen)
			}
			if tt.wantFile != "" && list[0].File != tt.wantFile {
				t.Errorf("list[0].File = %q, want %q", list[0].File, tt.wantFile)
			}
		})
	}
}

func TestParseFindingsResponse_proseThenFencedArray_droppedFindingsTraced(t *testing.T) {
	// Recovery goes through the same normalize/validate path: invalid findings
	// inside a recovered payload are dropped, valid ones kept, and OnDropped fires.
	resp := "Some analysis first.\n\n```json\n[" + recoveryFindingJSON + `,{"file":"y.py","line":1,"severity":"warning","category":"bug","message":""}]` + "\n```"
	var dropped []string
	list, err := ParseFindingsResponse(resp, ParseOptions{OnDropped: func(index int, reason string) {
		dropped = append(dropped, reason)
	}})
	if err != nil {
		t.Fatalf("ParseFindingsResponse: %v", err)
	}
	if len(list) != 1 || list[0].File != "x.py" {
		t.Fatalf("list = %+v, want 1 finding for x.py", list)
	}
	if len(dropped) != 1 {
		t.Errorf("dropped = %v, want exactly 1 dropped (empty message)", dropped)
	}
}

// TestExtractFindingsJSON_fencedBlocks exercises block collection and ordering
// directly: newest block first, unterminated fences yield no block.
func TestExtractFindingsJSON_fencedBlocks(t *testing.T) {
	payload := "[" + recoveryFindingJSON + "]"

	// No language tag.
	got, ok := extractFindingsJSON("prose\n```\n" + payload + "\n```\ntrailer")
	if !ok || trimSpace(got) != payload {
		t.Errorf("no-tag fence: ok=%v got=%q want %q", ok, got, payload)
	}

	// Unterminated fence: no fenced candidate, but the span fallback still recovers.
	got, ok = extractFindingsJSON("prose\n```json\n" + payload)
	if !ok || trimSpace(got) != payload {
		t.Errorf("unterminated fence: ok=%v got=%q want %q", ok, got, payload)
	}

	// Nothing recoverable.
	if got, ok := extractFindingsJSON("just prose, no JSON at all"); ok {
		t.Errorf("pure prose: ok=true got=%q, want false", got)
	}
}

func TestIsFindingsPayload_acceptsOnlyArraysAndWrappers(t *testing.T) {
	accept := []string{
		"[]",
		"[" + recoveryFindingJSON + "]",
		`{"findings":[]}`,
		`{"findings":[` + recoveryFindingJSON + `]}`,
		`{"other":1,"findings":[` + recoveryFindingJSON + `]}`, // wrapper with extra keys
	}
	for _, c := range accept {
		if !isFindingsPayload(c) {
			t.Errorf("isFindingsPayload(%q) = false, want true", c)
		}
	}
	reject := []string{
		"",
		recoveryFindingJSON, // single finding object: not accepted by the extractor
		`{"no_findings_key":[]}`,
		`{"findings":{"file":"x.py"}}`, // findings must be an array
		`[1,2,3]`,                      // array of non-objects
		`["a string"]`,
		`{"findings":"text"}`,
	}
	for _, c := range reject {
		if isFindingsPayload(c) {
			t.Errorf("isFindingsPayload(%q) = true, want false", c)
		}
	}
}

func TestBalancedJSONSpan_stringAwareness(t *testing.T) {
	s := `before {"msg":"has ] and } inside","n":1} after`
	span, ok := balancedJSONSpan(s, len("before "))
	if !ok {
		t.Fatal("balancedJSONSpan: want ok")
	}
	if want := `{"msg":"has ] and } inside","n":1}`; span != want {
		t.Errorf("span = %q, want %q", span, want)
	}
	// Escaped quote inside the string must not end the string early.
	s2 := `{"msg":"escaped \" bracket ] here"}`
	span, ok = balancedJSONSpan(s2, 0)
	if !ok || span != s2 {
		t.Errorf("span = %q ok=%v, want full span", span, ok)
	}
	// Unbalanced (truncated) input returns false.
	if _, ok := balancedJSONSpan(`[{"a":1`, 0); ok {
		t.Error("truncated span: want ok=false")
	}
}

func TestMergeFindingsFromParts_proseFirstPartFencedLastPart(t *testing.T) {
	parts := []string{
		"Looking at the hunk, I need to check the loop bounds and the nil check first.\n\nThe loop looks fine, let me verify the nil check.",
		"```json\n[{\"file\":\"m.go\",\"line\":7,\"severity\":\"warning\",\"category\":\"bug\",\"message\":\"nil map write\",\"confidence\":0.9}]\n```",
	}
	list, err := MergeFindingsFromParts(parts)
	if err != nil {
		t.Fatalf("MergeFindingsFromParts: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].File != "m.go" || list[0].Message != "nil map write" {
		t.Errorf("finding: %+v", list[0])
	}
}

func TestMergeFindingsFromParts_eachPartProseWithBareArray(t *testing.T) {
	// Two prose parts each with a bare array: merged via per-part recovery so
	// both findings survive (the joined string alone would only recover the last).
	parts := []string{
		"First part analysis.\n[" + recoveryFindingJSON + "]",
		"Second part analysis.\n[{\"file\":\"z.py\",\"line\":9,\"severity\":\"info\",\"category\":\"bug\",\"message\":\"second\",\"confidence\":0.5}]",
	}
	list, err := MergeFindingsFromParts(parts)
	if err != nil {
		t.Fatalf("MergeFindingsFromParts: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
	if list[0].File != "x.py" || list[1].File != "z.py" {
		t.Errorf("files: %q, %q", list[0].File, list[1].File)
	}
}

func TestParseFindingsResponse_recoveryAssignFindingIDs(t *testing.T) {
	// End-to-end shape from the Mission: recovered findings flow through
	// AssignFindingIDs like strict-parsed ones.
	resp := "Looking at the code, I need to check ...\n\n```json\n[" + recoveryFindingJSON + "]\n```"
	list, err := ParseFindingsResponse(resp)
	if err != nil {
		t.Fatalf("ParseFindingsResponse: %v", err)
	}
	out, err := AssignFindingIDs(list, "x.py")
	if err != nil {
		t.Fatalf("AssignFindingIDs: %v", err)
	}
	if len(out) != 1 || out[0].ID == "" {
		t.Fatalf("out = %+v, want 1 finding with ID", out)
	}
	if out[0].Confidence != 0.8 {
		t.Errorf("confidence = %v, want 0.8", out[0].Confidence)
	}
	if out[0].Severity != findings.SeverityWarning || out[0].Category != findings.CategoryBug {
		t.Errorf("severity/category: %q/%q", out[0].Severity, out[0].Category)
	}
}
