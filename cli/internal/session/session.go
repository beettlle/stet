// Package session provides the state schema and storage for a single review
// session: session.json with baseline_ref, last_reviewed_at, dismissed_ids,
// and optional prompt_shadows. Load/save and advisory lock live in this package.
package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"stet/cli/internal/erruser"
	"stet/cli/internal/findings"
)

// ErrLocked indicates the session lock is already held (e.g. another stet start).
var ErrLocked = errors.New("session already active")

const (
	sessionFilename = "session.json"
	lockFilename    = "lock"
)

// PromptShadow stores a finding_id and prompt_context for future negative
// few-shot injection (Phase 6). Stored in session now, not yet used.
type PromptShadow struct {
	FindingID     string `json:"finding_id"`
	PromptContext string `json:"prompt_context"`
}

// Session is the persisted state for one review session per repo.
// Stored at stateDir/session.json.
type Session struct {
	SessionID             string             `json:"session_id,omitempty"`
	BaselineRef           string             `json:"baseline_ref"`
	LastReviewedAt        string             `json:"last_reviewed_at"`
	DismissedIDs          []string           `json:"dismissed_ids,omitempty"`
	PromptShadows         []PromptShadow     `json:"prompt_shadows,omitempty"`
	FindingPromptContext  map[string]string  `json:"finding_prompt_context,omitempty"`
	Findings              []findings.Finding `json:"findings,omitempty"`
}

// Load reads the session from stateDir/session.json. If the file does not
// exist, returns a zero Session and nil error. If the file exists but
// contains invalid JSON, returns an error. Load does not create stateDir.
func Load(stateDir string) (Session, error) {
	path := filepath.Join(stateDir, sessionFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Session{}, nil
		}
		return Session{}, erruser.New("Could not read session file.", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return Session{}, erruser.New("Session file is invalid or corrupted.", err)
	}
	return s, nil
}

// Save writes the session to stateDir/session.json. Creates stateDir if
// needed. Uses atomic write (temp file then rename) to avoid corruption.
func Save(stateDir string, s *Session) error {
	if s == nil {
		return erruser.New("Cannot save nil session.", nil)
	}
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return erruser.New("Could not create session directory.", err)
	}
	path := filepath.Join(stateDir, sessionFilename)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return erruser.New("Could not save session.", err)
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "session.*.tmp")
	if err != nil {
		return erruser.New("Could not save session.", err)
	}
	tmpPath := f.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return erruser.New("Could not save session.", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return erruser.New("Could not save session.", err)
	}
	if err := f.Close(); err != nil {
		return erruser.New("Could not save session.", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return erruser.New("Could not save session.", err)
	}
	return nil
}
