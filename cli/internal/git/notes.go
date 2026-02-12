// Package git (notes.go) provides Git notes operations for the stet ref.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// NotesRefStet is the ref used for stet session notes (written on finish).
const NotesRefStet = "refs/notes/stet"

// AddNote writes a note to the given commit under notesRef. Overwrites any
// existing note at that commit (uses -f). repoRoot is the git repository root.
func AddNote(repoRoot, notesRef, commitRef, body string) error {
	if repoRoot == "" || notesRef == "" || commitRef == "" {
		return fmt.Errorf("git notes: repo root, notes ref, and commit ref required")
	}
	cmd := exec.Command("git", "notes", "--ref="+notesRef, "add", "-f", "-m", body, commitRef)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git notes add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// GetNote reads the note at the given commit under notesRef. Returns the note
// body, or an error if the note does not exist or the command fails.
func GetNote(repoRoot, notesRef, commitRef string) (string, error) {
	if repoRoot == "" || notesRef == "" || commitRef == "" {
		return "", fmt.Errorf("git notes: repo root, notes ref, and commit ref required")
	}
	cmd := exec.Command("git", "notes", "--ref="+notesRef, "show", commitRef)
	cmd.Dir = repoRoot
	cmd.Env = minimalEnv()
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", fmt.Errorf("git notes show: no note for %s", commitRef)
		}
		return "", fmt.Errorf("git notes show: %w", err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}
