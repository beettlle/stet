package diff

import (
	"strings"
	"testing"
)

func TestParseUnifiedDiff_empty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
	}{
		{"empty string", ""},
		{"whitespace only", "   \n\t\n  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUnifiedDiff(tt.in)
			if err != nil {
				t.Fatalf("ParseUnifiedDiff: %v", err)
			}
			if got != nil {
				t.Errorf("ParseUnifiedDiff = %v, want nil", got)
			}
		})
	}
}

func TestParseUnifiedDiff_singleFileSingleHunk(t *testing.T) {
	t.Parallel()
	diff := `diff --git a/foo.go b/foo.go
index abc123..def456 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 package main
+
 func main() {
 	println("hello")
`
	got, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(hunks) = %d, want 1", len(got))
	}
	if got[0].FilePath != "foo.go" {
		t.Errorf("FilePath = %q, want foo.go", got[0].FilePath)
	}
	if !strings.Contains(got[0].RawContent, "@@ -1,3 +1,4 @@") {
		t.Errorf("RawContent missing hunk header: %q", got[0].RawContent)
	}
	if !strings.Contains(got[0].RawContent, "package main") {
		t.Errorf("RawContent missing context: %q", got[0].RawContent)
	}
	if got[0].Context != got[0].RawContent {
		t.Error("Context should equal RawContent")
	}
}

func TestParseUnifiedDiff_multipleHunks(t *testing.T) {
	t.Parallel()
	diff := `diff --git a/x.go b/x.go
--- a/x.go
+++ b/x.go
@@ -1,2 +1,2 @@
-a
+b
@@ -5,1 +5,2 @@
 c
+d
`
	got, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(hunks) = %d, want 2", len(got))
	}
	if got[0].FilePath != "x.go" || got[1].FilePath != "x.go" {
		t.Errorf("FilePath: %q, %q", got[0].FilePath, got[1].FilePath)
	}
	if !strings.Contains(got[0].RawContent, "-a") || !strings.Contains(got[0].RawContent, "+b") {
		t.Errorf("first hunk content: %q", got[0].RawContent)
	}
	if !strings.Contains(got[1].RawContent, "c") || !strings.Contains(got[1].RawContent, "+d") {
		t.Errorf("second hunk content: %q", got[1].RawContent)
	}
}

func TestParseUnifiedDiff_multipleFiles(t *testing.T) {
	t.Parallel()
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,1 @@
-old
+new
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,1 +1,1 @@
-x
+y
`
	got, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(hunks) = %d, want 2", len(got))
	}
	if got[0].FilePath != "a.go" {
		t.Errorf("first FilePath = %q, want a.go", got[0].FilePath)
	}
	if got[1].FilePath != "b.go" {
		t.Errorf("second FilePath = %q, want b.go", got[1].FilePath)
	}
	if !strings.Contains(got[0].RawContent, "old") || !strings.Contains(got[0].RawContent, "new") {
		t.Errorf("first hunk: %q", got[0].RawContent)
	}
	if !strings.Contains(got[1].RawContent, "x") || !strings.Contains(got[1].RawContent, "y") {
		t.Errorf("second hunk: %q", got[1].RawContent)
	}
}

func TestParseUnifiedDiff_binarySkipped(t *testing.T) {
	t.Parallel()
	diff := `diff --git a/bin.dat b/bin.dat
index 111..222 100644
Binary files a/bin.dat and b/bin.dat differ
diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -1,1 +1,1 @@
-x
+y
`
	got, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(hunks) = %d, want 1 (binary file skipped)", len(got))
	}
	if got[0].FilePath != "foo.go" {
		t.Errorf("FilePath = %q, want foo.go", got[0].FilePath)
	}
}

func TestParseUnifiedDiff_binaryOnly(t *testing.T) {
	t.Parallel()
	diff := `diff --git a/bin b/bin
Binary files a/bin and b/bin differ
`
	got, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(hunks) = %d, want 0", len(got))
	}
}

func TestParseUnifiedDiff_usesBpath(t *testing.T) {
	t.Parallel()
	diff := `diff --git a/oldname.go b/newname.go
--- a/oldname.go
+++ b/newname.go
@@ -1,1 +1,1 @@
 same
`
	got, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(hunks) = %d, want 1", len(got))
	}
	if got[0].FilePath != "newname.go" {
		t.Errorf("FilePath = %q (should use b/ path), want newname.go", got[0].FilePath)
	}
}

// TestParseUnifiedDiff_noDiffGitLine covers parsePathLine: section without
// "diff --git " (e.g. minimal unified diff) gets path from ---/+++ lines.
func TestParseUnifiedDiff_noDiffGitLine(t *testing.T) {
	t.Parallel()
	diff := `--- a/standalone.go
+++ b/standalone.go
@@ -1,1 +1,1 @@
-x
+y
`
	got, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(hunks) = %d, want 1", len(got))
	}
	if got[0].FilePath != "standalone.go" {
		t.Errorf("FilePath = %q, want standalone.go", got[0].FilePath)
	}
}

// TestParseUnifiedDiff_pathWithTab covers parsePathLine when ---/+++ line has a tab (e.g. timestamp).
func TestParseUnifiedDiff_pathWithTab(t *testing.T) {
	t.Parallel()
	diff := `diff --git a/file.go b/file.go
--- a/file.go	2020-01-01 00:00:00
+++ b/file.go	2020-01-01 00:00:01
@@ -1,1 +1,1 @@
 x
`
	got, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(hunks) = %d, want 1", len(got))
	}
	if got[0].FilePath != "file.go" {
		t.Errorf("FilePath = %q, want file.go", got[0].FilePath)
	}
}

// TestParseUnifiedDiff_newFile covers /dev/null (trimDiffPath returns path unchanged when not a/ or b/).
func TestParseUnifiedDiff_newFile(t *testing.T) {
	t.Parallel()
	diff := `diff --git a/dev/null b/newfile.go
--- /dev/null
+++ b/newfile.go
@@ -0,0 +1,2 @@
+package main
+func main() {}
`
	got, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(hunks) = %d, want 1", len(got))
	}
	if got[0].FilePath != "newfile.go" {
		t.Errorf("FilePath = %q, want newfile.go", got[0].FilePath)
	}
}
