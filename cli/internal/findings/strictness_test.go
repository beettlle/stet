package findings

import (
	"testing"
)

func TestResolveStrictness(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		s        string
		wantKeep float64
		wantMaint float64
		wantFP   bool
		wantErr  bool
	}{
		{"strict", "strict", 0.6, 0.7, true, false},
		{"strict_plus", "strict+", 0.6, 0.7, false, false},
		{"default", "default", 0.8, 0.9, true, false},
		{"default_plus", "default+", 0.8, 0.9, false, false},
		{"lenient", "lenient", 0.9, 0.95, true, false},
		{"lenient_plus", "lenient+", 0.9, 0.95, false, false},
		{"strict_uppercase", "STRICT", 0.6, 0.7, true, false},
		{"default_whitespace", "  default  ", 0.8, 0.9, true, false},
		{"invalid", "aggressive", 0, 0, false, true},
		{"invalid_empty", "", 0, 0, false, true},
		{"invalid_typo", "stric", 0, 0, false, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotKeep, gotMaint, gotFP, err := ResolveStrictness(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveStrictness(%q): err = %v, wantErr = %v", tt.s, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if gotKeep != tt.wantKeep || gotMaint != tt.wantMaint {
				t.Errorf("ResolveStrictness(%q): thresholds = %g, %g; want %g, %g", tt.s, gotKeep, gotMaint, tt.wantKeep, tt.wantMaint)
			}
			if gotFP != tt.wantFP {
				t.Errorf("ResolveStrictness(%q): applyFP = %v, want %v", tt.s, gotFP, tt.wantFP)
			}
		})
	}
}
