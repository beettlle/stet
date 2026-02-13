package findings

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSetCursorURIs_emptySetFromRepoRootAndLine(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	list := []Finding{
		{File: "pkg/foo.go", Line: 10, Severity: SeverityInfo, Category: CategoryStyle, Confidence: 1.0, Message: "x"},
	}
	SetCursorURIs(repoRoot, list)
	if list[0].CursorURI == "" {
		t.Fatal("CursorURI should be set")
	}
	if !strings.HasPrefix(list[0].CursorURI, "file://") {
		t.Errorf("CursorURI should start with file://; got %q", list[0].CursorURI)
	}
	if !strings.Contains(list[0].CursorURI, "foo.go") {
		t.Errorf("CursorURI should contain file path; got %q", list[0].CursorURI)
	}
	if !strings.HasSuffix(list[0].CursorURI, "#L10") {
		t.Errorf("CursorURI should end with #L10; got %q", list[0].CursorURI)
	}
	absPath, _ := filepath.Abs(filepath.Join(repoRoot, "pkg/foo.go"))
	if !strings.Contains(list[0].CursorURI, filepath.ToSlash(absPath)) {
		t.Errorf("CursorURI should contain absolute path; got %q", list[0].CursorURI)
	}
}

func TestSetCursorURIs_nonEmptyNotOverwritten(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	existing := "file:///custom/path#L99"
	list := []Finding{
		{File: "a.go", Line: 1, CursorURI: existing, Severity: SeverityInfo, Category: CategoryStyle, Confidence: 1.0, Message: "x"},
	}
	SetCursorURIs(repoRoot, list)
	if list[0].CursorURI != existing {
		t.Errorf("existing CursorURI should not be overwritten; got %q", list[0].CursorURI)
	}
}

func TestSetCursorURIs_rangeUsedWhenPresent(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	list := []Finding{
		{File: "b.go", Line: 5, Range: &LineRange{Start: 5, End: 10}, Severity: SeverityWarning, Category: CategoryBug, Confidence: 1.0, Message: "y"},
	}
	SetCursorURIs(repoRoot, list)
	if list[0].CursorURI == "" {
		t.Fatal("CursorURI should be set")
	}
	if !strings.HasSuffix(list[0].CursorURI, "#L5-10") {
		t.Errorf("CursorURI should end with #L5-10 for range; got %q", list[0].CursorURI)
	}
}
