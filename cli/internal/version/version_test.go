package version

import "testing"

func TestString(t *testing.T) {
	// Save and restore globals so tests don't affect each other.
	savedVersion, savedCommit := Version, Commit
	defer func() { Version, Commit = savedVersion, savedCommit }()

	tests := []struct {
		name    string
		version string
		commit  string
		want    string
	}{
		{"dev with commit", "dev", "abc1234", "dev (abc1234)"},
		{"dev no commit", "dev", "", "dev"},
		{"release ignores commit", "v1.0.0", "abc1234", "v1.0.0"},
		{"release no commit", "v1.0.0", "", "v1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version, Commit = tt.version, tt.commit
			got := String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
