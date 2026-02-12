package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/findings"
)

func TestAppend_createsFileAndAppendsValidJSONL(t *testing.T) {
	dir := t.TempDir()
	rec := Record{
		DiffRef: "HEAD",
		ReviewOutput: []findings.Finding{
			{ID: "f1", File: "a.go", Line: 1, Severity: findings.SeverityWarning, Category: findings.CategoryStyle, Message: "msg"},
		},
		UserAction: UserAction{DismissedIDs: []string{"f1"}},
	}
	if err := Append(dir, rec, 0); err != nil {
		t.Fatalf("Append: %v", err)
	}
	path := filepath.Join(dir, historyFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Fatalf("want 1 non-empty line, got %d: %q", len(lines), lines)
	}
	var decoded Record
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.DiffRef != rec.DiffRef || len(decoded.ReviewOutput) != 1 || decoded.ReviewOutput[0].ID != "f1" {
		t.Errorf("decoded: %+v", decoded)
	}
}

func TestAppend_secondAppendAddsSecondLine(t *testing.T) {
	dir := t.TempDir()
	r1 := Record{DiffRef: "r1", ReviewOutput: nil, UserAction: UserAction{}}
	r2 := Record{DiffRef: "r2", ReviewOutput: nil, UserAction: UserAction{FinishedAt: "2025-01-01T00:00:00Z"}}
	if err := Append(dir, r1, 0); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if err := Append(dir, r2, 0); err != nil {
		t.Fatalf("Append 2: %v", err)
	}
	path := filepath.Join(dir, historyFilename)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	var d1, d2 Record
	if err := json.Unmarshal([]byte(lines[0]), &d1); err != nil {
		t.Fatalf("Unmarshal line 1: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &d2); err != nil {
		t.Fatalf("Unmarshal line 2: %v", err)
	}
	if d1.DiffRef != "r1" || d2.DiffRef != "r2" || d2.UserAction.FinishedAt != "2025-01-01T00:00:00Z" {
		t.Errorf("d1.DiffRef=%q d2.DiffRef=%q d2.FinishedAt=%q", d1.DiffRef, d2.DiffRef, d2.UserAction.FinishedAt)
	}
}

func TestAppend_rotationKeepsLastN(t *testing.T) {
	dir := t.TempDir()
	maxRecords := 2
	for i := 0; i < 3; i++ {
		rec := Record{DiffRef: string(rune('a' + i)), ReviewOutput: nil, UserAction: UserAction{}}
		if err := Append(dir, rec, maxRecords); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	path := filepath.Join(dir, historyFilename)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var count int
	for sc.Scan() {
		count++
		_ = sc.Text()
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if count != maxRecords {
		t.Errorf("after 3 appends with maxRecords=2: want 2 lines, got %d", count)
	}
	// Last two records should be "b" and "c"
	f2, _ := os.Open(path)
	defer f2.Close()
	sc2 := bufio.NewScanner(f2)
	var refs []string
	for sc2.Scan() {
		var r Record
		if err := json.Unmarshal(sc2.Bytes(), &r); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		refs = append(refs, r.DiffRef)
	}
	if len(refs) != 2 || refs[0] != "b" || refs[1] != "c" {
		t.Errorf("kept lines should be b and c, got %v", refs)
	}
}

func TestAppend_createsStateDir(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "nested", "review")
	rec := Record{DiffRef: "x", ReviewOutput: nil, UserAction: UserAction{}}
	if err := Append(stateDir, rec, 0); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, historyFilename)); err != nil {
		t.Errorf("history file not created: %v", err)
	}
}

func TestAppend_rotationWithMaxOne(t *testing.T) {
	dir := t.TempDir()
	if err := Append(dir, Record{DiffRef: "a", ReviewOutput: nil, UserAction: UserAction{}}, 1); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if err := Append(dir, Record{DiffRef: "b", ReviewOutput: nil, UserAction: UserAction{}}, 1); err != nil {
		t.Fatalf("Append 2: %v", err)
	}
	path := filepath.Join(dir, historyFilename)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var count int
	for sc.Scan() {
		count++
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if count != 1 {
		t.Errorf("after 2 appends with maxRecords=1: want 1 line, got %d", count)
	}
}

func TestAppend_stateDirIsFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "notadir")
	if err := os.WriteFile(stateDir, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	rec := Record{DiffRef: "x", ReviewOutput: nil, UserAction: UserAction{}}
	err := Append(stateDir, rec, 0)
	if err == nil {
		t.Fatal("Append with stateDir as file: want error")
	}
}

func TestAppend_readOnlyFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	rec := Record{DiffRef: "x", ReviewOutput: nil, UserAction: UserAction{}}
	if err := Append(dir, rec, 0); err != nil {
		t.Fatalf("first Append: %v", err)
	}
	historyPath := filepath.Join(dir, historyFilename)
	if err := os.Chmod(historyPath, 0444); err != nil {
		t.Skip("Chmod not supported or not effective on this platform")
	}
	defer func() { _ = os.Chmod(historyPath, 0644) }()
	err := Append(dir, rec, 0)
	if err == nil {
		t.Fatal("Append to read-only file: want error")
	}
}

func TestAppend_rotateIfNeededReadError(t *testing.T) {
	dir := t.TempDir()
	if err := Append(dir, Record{DiffRef: "a", ReviewOutput: nil, UserAction: UserAction{}}, 1); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	historyPath := filepath.Join(dir, historyFilename)
	if err := os.Chmod(historyPath, 0222); err != nil {
		t.Skip("Chmod not supported on this platform")
	}
	defer func() { _ = os.Chmod(historyPath, 0644) }()
	err := Append(dir, Record{DiffRef: "b", ReviewOutput: nil, UserAction: UserAction{}}, 1)
	if err == nil {
		t.Fatal("Append with unreadable file during rotation: want error")
	}
}

