package criticeval

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/findings"
)

// copyFixture copies testdata/<name> into a fresh temp dir and returns the
// dir path, so tests never write into the committed fixtures.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", name)
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", src, err)
	}
	dst := t.TempDir()
	for _, e := range entries {
		if e.IsDir() {
			t.Fatalf("fixture %s has unexpected subdir %s", name, e.Name())
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0644); err != nil {
			t.Fatalf("WriteFile(%s): %v", e.Name(), err)
		}
	}
	return dst
}

// dirHash returns a sha256 of every regular file under dir, keyed by
// relative path. Used to assert the loader is read-only.
func dirHash(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("hash dir %s: %v", dir, err)
	}
	return out
}

func groupByHunk(t *testing.T, d *Dataset) map[string]HunkGroup {
	t.Helper()
	m := make(map[string]HunkGroup, len(d.Groups))
	for _, g := range d.Groups {
		if _, dup := m[g.Hunk]; dup {
			t.Errorf("duplicate group for identical hunk text %q", g.Hunk)
		}
		m[g.Hunk] = g
	}
	return m
}

func findExample(t *testing.T, d *Dataset, id string) Example {
	t.Helper()
	for _, g := range d.Groups {
		for _, ex := range g.Examples {
			if ex.Finding.ID == id {
				return ex
			}
		}
	}
	t.Fatalf("finding %q not present in dataset groups", id)
	return Example{}
}

// loadFixtureHunk is the shared hunk text between history dismissal f1 and
// session finding s1 in the combined fixture.
const loadFixtureHunk = "func load(path string) error {\n\tdata, err := os.ReadFile(path)\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn parse(data)\n}"

func TestLoadFromState_combined(t *testing.T) {
	t.Parallel()
	dir := copyFixture(t, "state")
	before := dirHash(t, dir)

	d, err := LoadFromState(dir)
	if err != nil {
		t.Fatalf("LoadFromState: %v", err)
	}
	if d == nil {
		t.Fatal("LoadFromState returned nil dataset")
	}

	// Labels: drops from history (reason + prompt_context + resolvable ID),
	// keeps from session (active + prompt_context), exclusions applied.
	if got := d.Counts[LabelKeep]; got != 2 {
		t.Errorf("Counts[keep] = %d, want 2 (s1, s2)", got)
	}
	if got := d.Counts[LabelDrop]; got != 1 {
		t.Errorf("Counts[drop] = %d, want 1 (f6)", got)
	}
	if got := d.BySource[SourceHistory]; got != 1 {
		t.Errorf("BySource[history] = %d, want 1", got)
	}
	if got := d.BySource[SourceSession]; got != 2 {
		t.Errorf("BySource[session] = %d, want 2", got)
	}
	// f1 appears as drop in history and keep in session: conflicting labels
	// remove the ID entirely.
	if got := d.BySource[SourceConflict]; got != 1 {
		t.Errorf("BySource[conflict] = %d, want 1 (f1)", got)
	}
	for _, ex := range append([]Example{}, findExample(t, d, "s1"), findExample(t, d, "s2")) {
		if ex.Label != LabelKeep {
			t.Errorf("finding %q label = %q, want keep", ex.Finding.ID, ex.Label)
		}
		if ex.Source != SourceSession {
			t.Errorf("finding %q source = %q, want session", ex.Finding.ID, ex.Source)
		}
	}
	if ex := findExample(t, d, "f6"); ex.Label != LabelDrop || ex.Source != SourceHistory {
		t.Errorf("f6 label/source = %q/%q, want drop/history", ex.Label, ex.Source)
	}

	groups := groupByHunk(t, d)
	if len(d.Groups) != 3 {
		t.Errorf("len(Groups) = %d, want 3", len(d.Groups))
	}
	// History drop f1 (conflicted away) and session keep s1 share a hunk: the
	// group must contain only s1.
	if g, ok := groups[loadFixtureHunk]; !ok {
		t.Errorf("no group for shared load hunk")
	} else if len(g.Examples) != 1 || g.Examples[0].Finding.ID != "s1" {
		t.Errorf("load hunk group = %+v, want only s1", g.Examples)
	}
	// Excluded dismissal reasons never produce examples.
	for _, id := range []string{"f1", "f2", "f3", "f4", "f5"} {
		for _, g := range d.Groups {
			for _, ex := range g.Examples {
				if ex.Finding.ID == id {
					t.Errorf("finding %q should be excluded, found labeled %q", id, ex.Label)
				}
			}
		}
	}
	// Session finding s4 has no prompt context: not usable as an example.
	for _, g := range d.Groups {
		for _, ex := range g.Examples {
			if ex.Finding.ID == "s4" {
				t.Errorf("s4 (no prompt_context) should be skipped")
			}
		}
	}

	if after := dirHash(t, dir); !equalHashes(before, after) {
		t.Errorf("LoadFromState modified the state dir (read-only violation)")
	}
}

func equalHashes(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestLoadFromState_missingStateDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	d, err := LoadFromState(dir)
	if err != nil {
		t.Fatalf("LoadFromState(missing dir): %v", err)
	}
	if len(d.Groups) != 0 || d.Counts[LabelKeep] != 0 || d.Counts[LabelDrop] != 0 {
		t.Errorf("missing dir should yield empty dataset, got %+v", d)
	}
}

func TestLoadFromState_emptyStateDir(t *testing.T) {
	t.Parallel()
	d, err := LoadFromState(t.TempDir())
	if err != nil {
		t.Fatalf("LoadFromState(empty dir): %v", err)
	}
	if len(d.Groups) != 0 || d.Counts[LabelKeep] != 0 || d.Counts[LabelDrop] != 0 {
		t.Errorf("empty dir should yield empty dataset, got %+v", d)
	}
}

func TestLoadFromState_invalidHistoryPropagates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "history.jsonl"), []byte("{not json}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFromState(dir); err == nil {
		t.Errorf("LoadFromState(invalid history) = nil error, want parse error")
	}
}

func TestLoadDataset_valid(t *testing.T) {
	t.Parallel()
	d, err := LoadDataset(filepath.Join("testdata", "curated", "valid.jsonl"))
	if err != nil {
		t.Fatalf("LoadDataset(valid): %v", err)
	}
	if got := d.Counts[LabelKeep]; got != 2 {
		t.Errorf("Counts[keep] = %d, want 2", got)
	}
	if got := d.Counts[LabelDrop]; got != 2 {
		t.Errorf("Counts[drop] = %d, want 2", got)
	}
	if got := d.BySource[SourceCurated]; got != 4 {
		t.Errorf("BySource[curated] = %d, want 4", got)
	}
	groups := groupByHunk(t, d)
	if len(d.Groups) != 2 {
		t.Errorf("len(Groups) = %d, want 2 (grouped by identical hunk)", len(d.Groups))
	}
	sharedHunk := "func a() error {\n\treturn do()\n}"
	if g, ok := groups[sharedHunk]; !ok || len(g.Examples) != 2 {
		t.Errorf("shared hunk group = %+v, want 2 examples", g)
	} else {
		for _, ex := range g.Examples {
			if ex.Label != LabelKeep || ex.Source != SourceCurated {
				t.Errorf("shared hunk example %q label/source = %q/%q, want keep/curated", ex.Finding.ID, ex.Label, ex.Source)
			}
		}
	}
	// Line without finding.id still lands in its hunk group.
	otherHunk := "func c() int {\n\tx := 1\n\treturn x\n}"
	if g, ok := groups[otherHunk]; !ok || len(g.Examples) != 2 {
		t.Errorf("other hunk group = %+v, want 2 examples", g)
	}
}

func TestLoadDataset_conflictingLabels(t *testing.T) {
	t.Parallel()
	d, err := LoadDataset(filepath.Join("testdata", "curated", "conflict.jsonl"))
	if err != nil {
		t.Fatalf("LoadDataset(conflict): %v", err)
	}
	if len(d.Groups) != 0 || d.Counts[LabelKeep] != 0 || d.Counts[LabelDrop] != 0 {
		t.Errorf("conflicting ID should be removed entirely, got %+v", d)
	}
	if got := d.BySource[SourceConflict]; got != 1 {
		t.Errorf("BySource[conflict] = %d, want 1", got)
	}
}

func TestLoadDataset_unknownLabelNamesLine(t *testing.T) {
	t.Parallel()
	_, err := LoadDataset(filepath.Join("testdata", "curated", "unknownlabel.jsonl"))
	if err == nil {
		t.Fatal("unknown label: want error, got nil")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q should name line 2", err)
	}
}

func TestLoadDataset_emptyHunkNamesLine(t *testing.T) {
	t.Parallel()
	_, err := LoadDataset(filepath.Join("testdata", "curated", "emptyhunk.jsonl"))
	if err == nil {
		t.Fatal("empty hunk: want error, got nil")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Errorf("error %q should name line 1", err)
	}
}

func TestLoadDataset_invalidJSONNamesLine(t *testing.T) {
	t.Parallel()
	_, err := LoadDataset(filepath.Join("testdata", "curated", "badjson.jsonl"))
	if err == nil {
		t.Fatal("invalid JSON: want error, got nil")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("error %q should name line 3", err)
	}
}

func TestLoadDataset_missingFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "absent.jsonl")
	if _, err := LoadDataset(path); err == nil {
		t.Fatal("missing file: want error, got nil")
	}
}

func TestLoadDataset_blankLinesSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "blank.jsonl")
	content := "\n" + `{"finding":{"id":"b1","file":"a.go","severity":"info","category":"style","message":"m"},"hunk":"func b() {}","label":"keep"}` + "\n\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := LoadDataset(path)
	if err != nil {
		t.Fatalf("LoadDataset(blank lines): %v", err)
	}
	if d.Counts[LabelKeep] != 1 || len(d.Groups) != 1 {
		t.Errorf("blank lines should be skipped, got %+v", d)
	}
}

func TestEvidenceLinesStillAcceptedInCurated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.jsonl")
	content := `{"finding":{"id":"e1","file":"a.go","line":5,"severity":"warning","category":"bug","confidence":0.7,"message":"m","evidence_lines":"5, 6"},"hunk":"h","label":"keep"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := LoadDataset(path)
	if err != nil {
		t.Fatalf("LoadDataset(evidence string): %v", err)
	}
	ex := findExample(t, d, "e1")
	want := findings.EvidenceLines{5, 6}
	if len(ex.Finding.EvidenceLines) != 2 || ex.Finding.EvidenceLines[0] != want[0] || ex.Finding.EvidenceLines[1] != want[1] {
		t.Errorf("EvidenceLines = %v, want %v", ex.Finding.EvidenceLines, want)
	}
}
