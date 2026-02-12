// Package version holds the CLI version string. Default is "dev"; release
// builds can set it via: go build -ldflags "-X stet/cli/internal/version.Version=v1.0.0"
package version

// Version is the stet CLI version. Set at build time for releases.
var Version = "dev"
