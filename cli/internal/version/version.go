// Package version holds the CLI version string. Default is "dev"; release
// builds can set it via: go build -ldflags "-X stet/cli/internal/version.Version=v1.0.0"
// Commit is the short (7-char) git commit hash for dev builds; set by Makefile.
package version

// Version is the stet CLI version. Set at build time for releases.
var Version = "dev"

// Commit is the short git commit hash (e.g. 7 chars). Set at build time for dev builds via ldflags.
var Commit = ""

// String returns the version string for display (e.g. --version, git notes).
// For dev builds with Commit set, returns "dev (abc1234)"; otherwise returns Version.
func String() string {
	if Version != "dev" || Commit == "" {
		return Version
	}
	return Version + " (" + Commit + ")"
}
