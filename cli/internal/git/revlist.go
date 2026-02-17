// Package git (revlist.go) provides rev-list for walking commit ranges.
package git

import (
	"os/exec"
	"strings"

	"stet/cli/internal/erruser"
)

// RevList returns full commit SHAs for the range since..until (commits
// reachable from until but not from since). Empty range returns nil, nil.
// Invalid refs return an error.
func RevList(repoRoot, sinceRef, untilRef string) ([]string, error) {
	if repoRoot == "" || sinceRef == "" || untilRef == "" {
		return nil, erruser.New("rev-list: repo root, since, and until refs required", nil)
	}
	rangeSpec := sinceRef + ".." + untilRef
	cmd := exec.Command("git", "rev-list", rangeSpec)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return nil, erruser.New("Invalid ref or commit.", err)
		}
		return nil, erruser.New("Could not list commits in range.", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
