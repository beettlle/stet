// Package diff provides a pipeline to produce a list of hunks (file path, raw
// content, context) from a git diff between baseline and HEAD refs.
//
// # .gitignore
// Only tracked files are included in the diff. Ignored files are out of scope;
// git diff does not report them.
//
// # Binary files
// Binary files are excluded; no hunks are produced for them. Git reports
// "Binary files ... differ" and does not emit unified diff content.
//
// # Generated files
// By default, hunks for generated or vendored files are excluded. Default
// patterns include: *.pb.go, *_generated.go, *.min.js, package-lock.json,
// go.sum, paths under vendor/, and paths under any coverage/ directory (e.g.
// coverage/, extension/coverage/ lcov-report HTML) so generated coverage output
// is not reviewed. Pass Options to override or extend.
//
// # Empty diff
// When baseline..HEAD has no changes, Hunks returns a nil slice and no error.
//
// # Merge commits
// When HEAD is a merge commit, git diff baseline..HEAD shows the combined diff
// (changes from the first parent of HEAD to HEAD). No special handling is
// applied.
package diff

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"stet/cli/internal/erruser"
)

// Hunk is one diff block for review: a file path and the raw unified hunk
// content (including @@ line and context). Context is set to RawContent for
// use in prompts.
type Hunk struct {
	FilePath   string // path relative to repo root (HEAD side)
	RawContent string
	Context    string // same as RawContent for pipeline output; used when building prompts
}

// Options configures the diff pipeline. Nil means use default exclusions.
type Options struct {
	// ExcludePatterns overrides or extends the default generated-file patterns.
	// Each entry is a filepath.Match-style pattern (e.g. "*.pb.go").
	// If non-nil, only these patterns are used (defaults are not merged).
	ExcludePatterns []string
}

// defaultExcludePatterns are applied when Options is nil or ExcludePatterns is nil.
var defaultExcludePatterns = []string{
	"*.pb.go",
	"*_generated.go",
	"*.min.js",
	"package-lock.json",
	"go.sum",
	"vendor/*",
	"vendor/**/*",
	"coverage/*",
}

// Hunks runs git diff baseline..head from repoRoot, parses the unified diff,
// skips binary files, applies exclude patterns, and returns the list of hunks.
// Context is used for cancellation when running git.
func Hunks(ctx context.Context, repoRoot, baselineRef, headRef string, opts *Options) ([]Hunk, error) {
	if repoRoot == "" {
		return nil, erruser.New("Repository root is required.", nil)
	}
	if baselineRef == headRef {
		return nil, nil
	}
	patterns := defaultExcludePatterns
	if opts != nil && len(opts.ExcludePatterns) > 0 {
		patterns = opts.ExcludePatterns
	}

	out, err := runGitDiff(ctx, repoRoot, baselineRef, headRef)
	if err != nil {
		return nil, err
	}

	hunks, err := ParseUnifiedDiff(out)
	if err != nil {
		return nil, erruser.New("Could not parse diff output.", err)
	}

	filtered := filterByPatterns(hunks, patterns)
	if len(filtered) == 0 {
		return nil, nil
	}
	return filtered, nil
}

func runGitDiff(ctx context.Context, repoRoot, baseline, head string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-color", "--no-ext-diff", baseline+".."+head)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnvForRepo(repoRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", erruser.New("Could not compute diff between baseline and HEAD.", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out))))
	}
	return string(out), nil
}

func minimalEnv() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_TERMINAL_PROMPT=0",
	}
}

// minimalEnvForRepo returns a minimal env with GIT_DIR set so git uses the
// given repo regardless of process cwd (avoids "Not a git repository" when
// cwd or Dir is ambiguous, e.g. in worktrees or some CI).
func minimalEnvForRepo(repoRoot string) []string {
	gitDir := filepath.Join(repoRoot, ".git")
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_DIR=" + gitDir,
	}
}

// filterByPatterns returns hunks whose FilePath does not match any of the
// filepath.Match patterns. Patterns use forward slashes; we normalize
// FilePath to use forward slashes for matching.
func filterByPatterns(hunks []Hunk, patterns []string) []Hunk {
	if len(patterns) == 0 {
		return hunks
	}
	norm := filepath.ToSlash
	out := make([]Hunk, 0, len(hunks))
	for _, h := range hunks {
		path := norm(h.FilePath)
		excluded := false
		for _, p := range patterns {
			// filepath.Match does not support **; treat vendor and coverage as prefix match
			if strings.HasPrefix(p, "vendor") {
				if path == "vendor" || strings.HasPrefix(path, "vendor/") {
					excluded = true
					break
				}
				continue
			}
			if strings.HasPrefix(p, "coverage") {
				// Exclude top-level coverage/ and any path under a coverage directory (e.g. extension/coverage/)
				if path == "coverage" || strings.HasPrefix(path, "coverage/") || strings.Contains(path, "/coverage/") {
					excluded = true
					break
				}
				continue
			}
			ok, err := filepath.Match(p, path)
			if err == nil && ok {
				excluded = true
				break
			}
			// try matching against base name for simple patterns; skip malformed patterns
			ok, err = filepath.Match(p, filepath.Base(path))
			if err != nil {
				continue
			}
			if ok {
				excluded = true
				break
			}
		}
		if !excluded {
			out = append(out, h)
		}
	}
	return out
}
