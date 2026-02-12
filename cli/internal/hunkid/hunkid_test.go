package hunkid

import (
	"testing"
)

func TestStrictHunkID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		path    string
		content string
		wantEq  []int // indices of other rows that must get the same ID
	}{
		{
			name:    "same path and content",
			path:    "foo.go",
			content: "func bar() {}",
			wantEq:  []int{1},
		},
		{
			name:    "same path and content duplicate",
			path:    "foo.go",
			content: "func bar() {}",
			wantEq:  []int{0},
		},
		{
			name:    "crlf normalizes to same as lf",
			path:    "foo.go",
			content: "func bar() {}\r\n",
			wantEq:  []int{3},
		},
		{
			name:    "lf only",
			path:    "foo.go",
			content: "func bar() {}\n",
			wantEq:  []int{2},
		},
		{
			name:    "different content different id",
			path:    "foo.go",
			content: "func baz() {}",
			wantEq:  nil,
		},
		{
			name:    "different path different id",
			path:    "bar.go",
			content: "func bar() {}",
			wantEq:  nil,
		},
		{
			name:    "empty path and content deterministic",
			path:    "",
			content: "",
			wantEq:  []int{7},
		},
		{
			name:    "empty path and content duplicate",
			path:    "",
			content: "",
			wantEq:  []int{6},
		},
	}
	ids := make([]string, len(tests))
	for i, tt := range tests {
		ids[i] = StrictHunkID(tt.path, tt.content)
	}
	for i, tt := range tests {
		for _, j := range tt.wantEq {
			if ids[i] != ids[j] {
				t.Errorf("%s: StrictHunkID(%q, %q) = %q, want same as row %d (%q)", tt.name, tt.path, tt.content, ids[i], j, ids[j])
			}
		}
	}
	// Different content/path must differ from first row where applicable
	if ids[0] == ids[4] {
		t.Error("different content should yield different strict ID")
	}
	if ids[0] == ids[5] {
		t.Error("different path should yield different strict ID")
	}
}

func TestStrictHunkID_CRLFEqualsLF(t *testing.T) {
	t.Parallel()
	path := "x.go"
	withLF := "a\nb\n"
	withCRLF := "a\r\nb\r\n"
	idLF := StrictHunkID(path, withLF)
	idCRLF := StrictHunkID(path, withCRLF)
	if idLF != idCRLF {
		t.Errorf("CRLF vs LF: strict ID %q != %q", idLF, idCRLF)
	}
}

func TestSemanticHunkID(t *testing.T) {
	t.Parallel()
	// Build semantic IDs for rows; assert same/different by index.
	type row struct {
		path    string
		content string
	}
	rows := []row{
		{"a.go", "func f() { x := 1 }"},
		{"a.go", "func f() { x := 1 } // comment"},
		{"a.go", "func  f()  {  x  :=  1  }"},
		{"b.go", "func f() { x := 1 }"},
		{"a.go", "func g() { y := 2 }"},
		{"file.xyz", "  a  b  "},
	}
	semanticIDs := make([]string, len(rows))
	for i, r := range rows {
		semanticIDs[i] = SemanticHunkID(r.path, r.content)
	}
	strictIDs := make([]string, len(rows))
	for i, r := range rows {
		strictIDs[i] = StrictHunkID(r.path, r.content)
	}

	// sameAs: row 1 (comment) must have same semantic as row 0
	if semanticIDs[1] != semanticIDs[0] {
		t.Errorf("go comment-only: semantic ID %q != %q (want same)", semanticIDs[1], semanticIDs[0])
	}
	if strictIDs[1] == strictIDs[0] {
		t.Error("go comment-only: strict ID should differ")
	}
	// row 2 (whitespace) same semantic as row 0
	if semanticIDs[2] != semanticIDs[0] {
		t.Errorf("go whitespace: semantic ID %q != %q", semanticIDs[2], semanticIDs[0])
	}
	// row 3 different path
	if semanticIDs[3] == semanticIDs[0] {
		t.Error("different path should yield different semantic ID")
	}
	// row 4 different code
	if semanticIDs[4] == semanticIDs[0] {
		t.Error("different code should yield different semantic ID")
	}
	// row 5 unknown ext: whitespace collapse only, deterministic
	if semanticIDs[5] != SemanticHunkID("file.xyz", "a b") {
		t.Error("unknown extension: semantic ID should be deterministic (whitespace collapse)")
	}
}

func TestSemanticHunkID_Python(t *testing.T) {
	t.Parallel()
	id1 := SemanticHunkID("x.py", "x = 1\n")
	id2 := SemanticHunkID("x.py", "x = 1  # comment\n")
	if id1 != id2 {
		t.Errorf("Python: comment-only change should yield same semantic ID: %q != %q", id1, id2)
	}
}

func TestStableFindingID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		file       string
		line       int
		rangeStart int
		rangeEnd   int
		message    string
		wantEq     []int
	}{
		{
			name:    "same inputs same id",
			file:    "a.go",
			line:    10,
			message: "bug here",
			wantEq:  []int{1},
		},
		{
			name:    "same inputs duplicate",
			file:    "a.go",
			line:    10,
			message: "bug here",
			wantEq:  []int{0},
		},
		{
			name:    "message stem normalized",
			file:    "a.go",
			line:    10,
			message: "  bug   here  ",
			wantEq:  []int{0, 1},
		},
		{
			name:    "different file",
			file:    "b.go",
			line:    10,
			message: "bug here",
			wantEq:  nil,
		},
		{
			name:    "different line",
			file:    "a.go",
			line:    11,
			message: "bug here",
			wantEq:  nil,
		},
		{
			name:    "different message",
			file:    "a.go",
			line:    10,
			message: "other",
			wantEq:  nil,
		},
		{
			name:       "range used when both positive",
			file:       "a.go",
			line:       5,
			rangeStart: 10,
			rangeEnd:   12,
			message:    "msg",
			wantEq:     []int{7},
		},
		{
			name:       "range duplicate",
			file:       "a.go",
			line:       5,
			rangeStart: 10,
			rangeEnd:   12,
			message:    "msg",
			wantEq:     []int{6},
		},
		{
			name:    "empty message deterministic",
			file:    "x.go",
			line:    1,
			message: "",
			wantEq:  []int{9},
		},
		{
			name:    "empty message duplicate",
			file:    "x.go",
			line:    1,
			message: "",
			wantEq:  []int{8},
		},
	}
	ids := make([]string, len(tests))
	for i, tt := range tests {
		ids[i] = StableFindingID(tt.file, tt.line, tt.rangeStart, tt.rangeEnd, tt.message)
	}
	for i, tt := range tests {
		for _, j := range tt.wantEq {
			if ids[i] != ids[j] {
				t.Errorf("%s: StableFindingID(...) = %q, want same as row %d (%q)", tt.name, ids[i], j, ids[j])
			}
		}
	}
	// Range vs line: row 6/7 use range; row 0 uses line
	if ids[6] == ids[0] {
		t.Error("range-based location should differ from line-only location")
	}
}

func TestStableFindingID_LineUsedWhenRangeInvalid(t *testing.T) {
	t.Parallel()
	// rangeStart > rangeEnd: use line
	id := StableFindingID("f.go", 10, 20, 5, "msg")
	idLine := StableFindingID("f.go", 10, 0, 0, "msg")
	if id != idLine {
		t.Errorf("invalid range should fall back to line: %q != %q", id, idLine)
	}
}

func TestNormalizeCRLF(t *testing.T) {
	t.Parallel()
	if got := normalizeCRLF("a\r\nb"); got != "a\nb" {
		t.Errorf("normalizeCRLF = %q, want a\nb", got)
	}
	if got := normalizeCRLF("a\nb"); got != "a\nb" {
		t.Errorf("normalizeCRLF(LF only) = %q", got)
	}
}

func TestMessageStem(t *testing.T) {
	t.Parallel()
	if got := messageStem("  a  b  "); got != "a b" {
		t.Errorf("messageStem = %q, want \"a b\"", got)
	}
}

func TestLangFromPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{"x.go", "go"},
		{"pkg/main.py", "python"},
		{"file.js", "js"},
		{"file.ts", "js"},
		{"file.tsx", "js"},
		{"script.sh", "sh"},
		{"file.xyz", ""},
	}
	for _, tt := range tests {
		got := langFromPath(tt.path)
		if got != tt.want {
			t.Errorf("langFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestCodeOnly_Go(t *testing.T) {
	t.Parallel()
	in := "a := 1 // comment\nb := 2"
	got := codeOnly(in, "go")
	want := "a := 1 b := 2"
	if got != want {
		t.Errorf("codeOnly(go) = %q, want %q", got, want)
	}
}

func TestCodeOnly_BlockComment(t *testing.T) {
	t.Parallel()
	in := "x/* block */y"
	got := codeOnly(in, "go")
	want := "x y"
	if got != want {
		t.Errorf("codeOnly(block) = %q, want %q", got, want)
	}
}
