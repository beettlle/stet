// Tests for the praise-only findings filter.
package findings

import (
	"reflect"
	"testing"
)

func TestFilterPraiseOnly(t *testing.T) {
	t.Parallel()
	praiseInfo := Finding{File: "a.go", Line: 1, Severity: SeverityInfo, Category: CategoryCorrectness, Confidence: 0.95, Message: "This correctly handles the nil case."}
	praiseNitpick := Finding{File: "a.go", Line: 2, Severity: SeverityNitpick, Category: CategoryStyle, Confidence: 0.9, Message: "Looks good to me."}
	infoWithSuggestion := Finding{File: "b.go", Line: 3, Severity: SeverityInfo, Category: CategoryMaintainability, Confidence: 0.9, Message: "Good use of a helper here.", Suggestion: "Also reuse it in parseB."}
	warningPraise := Finding{File: "c.go", Line: 4, Severity: SeverityWarning, Category: CategoryCorrectness, Confidence: 0.9, Message: "This correctly handles X."}
	errorPraise := Finding{File: "c.go", Line: 5, Severity: SeverityError, Category: CategoryBug, Confidence: 0.9, Message: "Nice refactor."}
	infoNoPraise := Finding{File: "d.go", Line: 6, Severity: SeverityInfo, Category: CategoryPerformance, Confidence: 0.9, Message: "Allocation inside the loop on every iteration."}
	tests := []struct {
		name   string
		input  []Finding
		output []Finding
	}{
		{name: "nil_input", input: nil, output: nil},
		{name: "empty_slice", input: []Finding{}, output: nil},
		{name: "info_praise_no_suggestion_dropped", input: []Finding{praiseInfo}, output: []Finding{}},
		{name: "nitpick_praise_dropped", input: []Finding{praiseNitpick}, output: []Finding{}},
		{
			name:   "whitespace_suggestion_dropped",
			input:  []Finding{{Severity: SeverityInfo, Message: "Well done.", Suggestion: "  \n\t"}},
			output: []Finding{},
		},
		{
			name:   "case_insensitive_dropped",
			input:  []Finding{{Severity: SeverityInfo, Message: "THE CHECK IS CORRECT"}},
			output: []Finding{},
		},
		{name: "info_with_suggestion_kept", input: []Finding{infoWithSuggestion}, output: []Finding{infoWithSuggestion}},
		{name: "warning_praise_kept", input: []Finding{warningPraise}, output: []Finding{warningPraise}},
		{name: "error_praise_kept", input: []Finding{errorPraise}, output: []Finding{errorPraise}},
		{name: "info_no_praise_kept", input: []Finding{infoNoPraise}, output: []Finding{infoNoPraise}},
		{
			name:   "incorrectly_not_praise_kept",
			input:  []Finding{{Severity: SeverityInfo, Message: "The offset is incorrectly computed."}},
			output: []Finding{{Severity: SeverityInfo, Message: "The offset is incorrectly computed."}},
		},
		{
			name:   "negated_praise_kept",
			input:  []Finding{{Severity: SeverityInfo, Message: "Does not correctly handle an empty slice."}},
			output: []Finding{{Severity: SeverityInfo, Message: "Does not correctly handle an empty slice."}},
		},
		{
			name:   "praise_with_caveat_kept",
			input:  []Finding{{Severity: SeverityInfo, Message: "Correctly parses the header, but the body length is unchecked."}},
			output: []Finding{{Severity: SeverityInfo, Message: "Correctly parses the header, but the body length is unchecked."}},
		},
		{
			name:   "mixed_order_preserved",
			input:  []Finding{infoNoPraise, praiseInfo, warningPraise, praiseNitpick, infoWithSuggestion},
			output: []Finding{infoNoPraise, warningPraise, infoWithSuggestion},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterPraiseOnly(tt.input)
			if tt.output == nil {
				if got != nil {
					t.Fatalf("FilterPraiseOnly = %#v; want nil", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("FilterPraiseOnly = %#v; want %#v", got, tt.output)
			}
		})
	}
}

func TestFilterPraiseOnly_doesNotMutateInput(t *testing.T) {
	t.Parallel()
	input := []Finding{
		{File: "a.go", Line: 1, Severity: SeverityInfo, Message: "This correctly handles X."},
		{File: "b.go", Line: 2, Severity: SeverityWarning, Message: "Possible nil dereference."},
	}
	snapshot := make([]Finding, len(input))
	copy(snapshot, input)
	got := FilterPraiseOnly(input)
	if !reflect.DeepEqual(input, snapshot) {
		t.Errorf("input mutated: got %#v; want %#v", input, snapshot)
	}
	if len(got) != 1 || got[0].File != "b.go" {
		t.Fatalf("FilterPraiseOnly = %#v; want only b.go", got)
	}
	got[0].Message = "changed"
	if input[1].Message != "Possible nil dereference." {
		t.Errorf("output aliases input: input[1].Message = %q", input[1].Message)
	}
}
