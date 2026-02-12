package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_missingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.BaselineRef != "" || got.LastReviewedAt != "" || len(got.DismissedIDs) != 0 || len(got.PromptShadows) != 0 {
		t.Errorf("Load(missing) = %+v, want zero Session", got)
	}
}

func TestLoad_invalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, sessionFilename)
	if err := os.WriteFile(path, []byte(`{invalid`), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load: expected error for invalid JSON")
	}
}

func TestSaveLoad_roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	want := Session{
		BaselineRef:    "main",
		LastReviewedAt: "abc123",
		DismissedIDs:   []string{"f1", "f2"},
		PromptShadows:  []PromptShadow{{FindingID: "f1", PromptContext: "ctx1"}},
	}
	if err := Save(dir, &want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.BaselineRef != want.BaselineRef {
		t.Errorf("BaselineRef = %q, want %q", got.BaselineRef, want.BaselineRef)
	}
	if got.LastReviewedAt != want.LastReviewedAt {
		t.Errorf("LastReviewedAt = %q, want %q", got.LastReviewedAt, want.LastReviewedAt)
	}
	if len(got.DismissedIDs) != len(want.DismissedIDs) {
		t.Errorf("DismissedIDs len = %d, want %d", len(got.DismissedIDs), len(want.DismissedIDs))
	}
	for i := range want.DismissedIDs {
		if i >= len(got.DismissedIDs) || got.DismissedIDs[i] != want.DismissedIDs[i] {
			t.Errorf("DismissedIDs[%d] = %v, want %v", i, got.DismissedIDs, want.DismissedIDs)
			break
		}
	}
	if len(got.PromptShadows) != len(want.PromptShadows) {
		t.Errorf("PromptShadows len = %d, want %d", len(got.PromptShadows), len(want.PromptShadows))
	}
	for i := range want.PromptShadows {
		if i >= len(got.PromptShadows) || got.PromptShadows[i] != want.PromptShadows[i] {
			t.Errorf("PromptShadows[%d] = %+v, want %+v", i, got.PromptShadows, want.PromptShadows)
			break
		}
	}
}

func TestSaveLoad_emptySession(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	empty := Session{}
	if err := Save(dir, &empty); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.BaselineRef != "" || got.LastReviewedAt != "" || len(got.DismissedIDs) != 0 || len(got.PromptShadows) != 0 {
		t.Errorf("Load(empty roundtrip) = %+v, want zero Session", got)
	}
}

func TestSave_nilSession(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := Save(dir, nil)
	if err == nil {
		t.Fatal("Save(nil): expected error")
	}
}

func TestSave_mkdirAllFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	readOnly := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnly, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnly, 0); err != nil {
		t.Skip("chmod 0 not supported or not permitted")
	}
	defer os.Chmod(readOnly, 0700)
	// Saving to a path under read-only dir should fail at MkdirAll
	stateDir := filepath.Join(readOnly, "sub", "state")
	err := Save(stateDir, &Session{BaselineRef: "main"})
	if err == nil {
		t.Fatal("Save: expected error when state dir cannot be created")
	}
}

func TestAcquireLock_releaseThenReacquire(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	release, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	release()
	release2, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock after release: %v", err)
	}
	release2()
}

func TestAcquireLock_secondCallFailsWithErrLocked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	release, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer release()
	_, err = AcquireLock(dir)
	if err == nil {
		t.Fatal("second AcquireLock: expected error")
	}
	if !errors.Is(err, ErrLocked) {
		t.Errorf("second AcquireLock: got %v, want ErrLocked", err)
	}
}
