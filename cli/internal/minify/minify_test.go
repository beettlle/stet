package minify

import (
	"strings"
	"testing"
)

func TestMinifyGoHunkContent(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty returns empty",
			in:   "",
			want: "",
		},
		{
			name: "single hunk header only",
			in:   "@@ -1,3 +1,4 @@",
			want: "@@ -1,3 +1,4 @@",
		},
		{
			name: "header plus lines with leading whitespace",
			in:   "@@ -1,3 +1,4 @@\n context\n-\told\n+\tnew",
			want: "@@ -1,3 +1,4 @@\n context\n-old\n+new",
		},
		{
			name: "multiple spaces collapsed",
			in:   "@@ -1,1 +1,2 @@\n  x :=   y  +  z",
			want: "@@ -1,1 +1,2 @@\n x := y + z",
		},
		{
			name: "prefix preserved",
			in:   "@@ -2,2 +2,3 @@\n \treturn nil\n-\treturn err\n+\treturn nil",
			want: "@@ -2,2 +2,3 @@\n return nil\n-return err\n+return nil",
		},
		{
			name: "no header returns original",
			in:   "not a hunk header\n line",
			want: "not a hunk header\n line",
		},
		{
			name: "blank lines preserved",
			in:   "@@ -1,2 +1,3 @@\n a\n\n b",
			want: "@@ -1,2 +1,3 @@\n a\n\n b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MinifyGoHunkContent(tt.in)
			if got != tt.want {
				t.Errorf("MinifyGoHunkContent() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestMinifyGoHunkContent_RealisticHunk(t *testing.T) {
	input := `@@ -10,5 +10,6 @@
 func foo() {
-	return 0
+	return 1
 }
`
	got := MinifyGoHunkContent(input)
	if !strings.HasPrefix(got, "@@ -10,5 +10,6 @@") {
		t.Errorf("expected hunk header preserved, got %q", got)
	}
	if strings.Contains(got, "\t") {
		t.Errorf("expected leading tabs removed from line bodies, got %q", got)
	}
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		if len(line) > 0 {
			first := line[0]
			if first != ' ' && first != '-' && first != '+' {
				t.Errorf("line %d: expected prefix space/-/+, got %q", i, line)
			}
		}
	}
}

func TestMinifyRustHunkContent(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty returns empty",
			in:   "",
			want: "",
		},
		{
			name: "single hunk header only",
			in:   "@@ -1,3 +1,4 @@",
			want: "@@ -1,3 +1,4 @@",
		},
		{
			name: "header plus lines with leading whitespace",
			in:   "@@ -1,3 +1,4 @@\n context\n-\told\n+\tnew",
			want: "@@ -1,3 +1,4 @@\n context\n-old\n+new",
		},
		{
			name: "multiple spaces collapsed",
			in:   "@@ -1,1 +1,2 @@\n  let x =   y  +  z;",
			want: "@@ -1,1 +1,2 @@\n let x = y + z;",
		},
		{
			name: "prefix preserved",
			in:   "@@ -2,2 +2,3 @@\n \tOk(())\n-\tErr(e)\n+\tOk(())",
			want: "@@ -2,2 +2,3 @@\n Ok(())\n-Err(e)\n+Ok(())",
		},
		{
			name: "no header returns original",
			in:   "not a hunk header\n line",
			want: "not a hunk header\n line",
		},
		{
			name: "blank lines preserved",
			in:   "@@ -1,2 +1,3 @@\n a\n\n b",
			want: "@@ -1,2 +1,3 @@\n a\n\n b",
		},
		{
			name: "Rust fn with extra spaces",
			in:   "@@ -5,4 +5,5 @@\n \tfn   foo()  { }\n-\t  bar();\n+\t  bar();",
			want: "@@ -5,4 +5,5 @@\n fn foo() { }\n-bar();\n+bar();",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MinifyRustHunkContent(tt.in)
			if got != tt.want {
				t.Errorf("MinifyRustHunkContent() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestMinifyRustHunkContent_RealisticHunk(t *testing.T) {
	input := `@@ -10,5 +10,6 @@
 fn bar() {
-    0
+    1
 }
`
	got := MinifyRustHunkContent(input)
	if !strings.HasPrefix(got, "@@ -10,5 +10,6 @@") {
		t.Errorf("expected hunk header preserved, got %q", got)
	}
	if strings.Contains(got, "\t") {
		t.Errorf("expected leading tabs removed from line bodies, got %q", got)
	}
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		if len(line) > 0 {
			first := line[0]
			if first != ' ' && first != '-' && first != '+' {
				t.Errorf("line %d: expected prefix space/-/+, got %q", i, line)
			}
		}
	}
}

func TestCollapseSpaces(t *testing.T) {
	if got := collapseSpaces("a  b\t\tc"); got != "a b c" {
		t.Errorf("collapseSpaces = %q, want %q", got, "a b c")
	}
	if got := collapseSpaces("x"); got != "x" {
		t.Errorf("collapseSpaces = %q, want %q", got, "x")
	}
	if got := collapseSpaces(""); got != "" {
		t.Errorf("collapseSpaces = %q, want %q", got, "")
	}
}
