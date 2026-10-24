// Tests for the existence-claim post-filter: findings claiming a symbol is
// missing, unused, undefined, not imported, or not exported are dropped only
// when the file content disproves the claim (issue #33).
package findings

import (
	"reflect"
	"testing"
)

// existenceJSFixture reproduces the pi-spine release examples from issue #33:
// SP-829 (fs imported while the reviewer claimed otherwise) and SP-831
// (compareTaskIds declared while claimed missing).
const existenceJSFixture = `import fs from "node:fs";
import { readFile } from "node:fs";

function createRulesFixture() {
  fs.mkdirSync("tmp");
  fs.rmSync("tmp", { recursive: true });
  return readFile;
}

function compareTaskIds(a, b) {
  return a === b;
}
`

// existenceGoFixture covers Go declarations, imports, and exported names.
const existenceGoFixture = `package main

import "fmt"

func CompareTaskIds(a, b string) bool {
	fmt.Println(RateLimit)
	fmt.Println(RateLimit)
	return a == b
}

var RateLimit = 10
`

// existencePyFixture covers Python def/class/import forms.
const existencePyFixture = `import os
import sys

from collections import defaultdict


def compare_task_ids(a, b):
    return a == b


class TaskRunner:
    def run(self):
        return os.getcwd()
`

func TestFilterExistenceClaims(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   []Finding
		content string
		keep    bool // whether the single input finding survives
	}{
		// --- empty inputs ---
		{
			name:    "empty_batch",
			input:   nil,
			content: existenceJSFixture,
			keep:    false, // nil in, nil out; covered by len assertion below
		},
		{
			name: "empty_content_keeps_everything",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is not defined."},
			},
			content: "",
			keep:    true,
		},
		{
			name: "whitespace_content_keeps_everything",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is not defined."},
			},
			content: "   \n\t\n",
			keep:    true,
		},

		// --- no existence claim: always kept ---
		{
			name: "no_claim_kept_even_when_identifiers_declared",
			input: []Finding{
				{File: "a.go", Line: 4, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "Possible nil dereference in `createRulesFixture`."},
			},
			content: existenceJSFixture,
			keep:    true,
		},
		{
			name: "value_use_of_undefined_is_not_a_claim_kept",
			input: []Finding{
				{File: "a.go", Line: 4, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "`createRulesFixture` returns undefined when the fixture list is empty."},
			},
			content: existenceJSFixture,
			keep:    true,
		},

		// --- no identifiers extractable: kept ---
		{
			name: "claim_without_backticked_identifier_kept",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "Missing import statement for the fs package."},
			},
			content: existenceJSFixture,
			keep:    true,
		},
		{
			name: "compound_path_identifier_kept",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`fs.promises` is not defined."},
			},
			content: existenceJSFixture,
			keep:    true,
		},

		// --- not defined / not imported / undefined (SP-829, SP-831) ---
		{
			name: "sp831_compareTaskIds_not_defined_dropped",
			input: []Finding{
				{File: "a.go", Line: 10, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is not defined."},
			},
			content: existenceJSFixture,
			keep:    false,
		},
		{
			name: "sp829_fs_without_importing_dropped",
			input: []Finding{
				{File: "a.go", Line: 5, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`createRulesFixture` uses `fs` without importing it."},
			},
			content: existenceJSFixture,
			keep:    false,
		},
		{
			name: "undefined_case_insensitive_dropped",
			input: []Finding{
				{File: "a.go", Line: 5, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is UNDEFINED at runtime."},
			},
			content: existenceJSFixture,
			keep:    false, // claim words match case-insensitively; identifiers match exactly
		},
		{
			name: "named_import_member_not_imported_dropped",
			input: []Finding{
				{File: "a.go", Line: 5, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`readFile` is not imported in this file."},
			},
			content: existenceJSFixture,
			keep:    false,
		},
		{
			name: "require_binding_not_imported_dropped",
			input: []Finding{
				{File: "a.js", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`path` is not imported."},
			},
			content: "const path = require(\"path\");\nmodule.exports = path;\n",
			keep:    false,
		},
		{
			name: "symbol_genuinely_absent_kept",
			input: []Finding{
				{File: "a.go", Line: 10, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`missingHelper` is not defined; define it before use."},
			},
			content: existenceJSFixture,
			keep:    true,
		},
		{
			name: "one_of_two_identifiers_unproven_kept",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` and `missingUtil` are not defined."},
			},
			content: existenceJSFixture,
			keep:    true,
		},
		{
			name: "referenceerror_declared_dropped",
			input: []Finding{
				{File: "a.go", Line: 4, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "Calling `compareTaskIds` here raises ReferenceError."},
			},
			content: existenceJSFixture,
			keep:    false,
		},
		{
			name: "referenceerror_absent_kept",
			input: []Finding{
				{File: "a.go", Line: 4, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "Calling `renderChart` here raises ReferenceError."},
			},
			content: existenceJSFixture,
			keep:    true,
		},
		{
			name: "declared_parameter_flagged_undefined_dropped",
			input: []Finding{
				{File: "handler.js", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`config` is undefined at the call site."},
			},
			content: "function handler(config) {\n  return config;\n}\n",
			keep:    false,
		},
		{
			name: "go_func_not_defined_dropped",
			input: []Finding{
				{File: "main.go", Line: 6, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`CompareTaskIds` is not defined."},
			},
			content: existenceGoFixture,
			keep:    false,
		},
		{
			name: "go_import_not_imported_dropped",
			input: []Finding{
				{File: "main.go", Line: 6, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`fmt` is not imported."},
			},
			content: existenceGoFixture,
			keep:    false,
		},
		{
			name: "go_param_flagged_undefined_dropped",
			input: []Finding{
				{File: "main.go", Line: 6, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "Parameter `b` is undefined."},
			},
			content: existenceGoFixture,
			keep:    false,
		},
		{
			name: "go_genuinely_absent_kept",
			input: []Finding{
				{File: "main.go", Line: 6, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is not defined."},
			},
			content: existenceGoFixture,
			keep:    true, // Go identifiers are case-sensitive: lowercase name absent
		},
		{
			name: "python_def_not_defined_dropped",
			input: []Finding{
				{File: "main.py", Line: 7, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compare_task_ids` is not defined."},
			},
			content: existencePyFixture,
			keep:    false,
		},
		{
			name: "python_class_not_defined_dropped",
			input: []Finding{
				{File: "main.py", Line: 7, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`TaskRunner` is not defined."},
			},
			content: existencePyFixture,
			keep:    false,
		},
		{
			name: "python_from_import_not_imported_dropped",
			input: []Finding{
				{File: "main.py", Line: 4, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`defaultdict` is not imported."},
			},
			content: existencePyFixture,
			keep:    false,
		},

		// --- unused ---
		{
			name: "used_import_flagged_unused_dropped",
			input: []Finding{
				{File: "a.go", Line: 2, Severity: SeverityWarning, Category: CategoryMaintainability, Confidence: 0.9, Message: "The `fs` import is unused."},
			},
			content: existenceJSFixture,
			keep:    false,
		},
		{
			name: "genuinely_unused_import_kept",
			input: []Finding{
				{File: "a.go", Line: 2, Severity: SeverityWarning, Category: CategoryMaintainability, Confidence: 0.9, Message: "The `sys` import is unused."},
			},
			content: existencePyFixture,
			keep:    true, // sys appears only on its import line
		},
		{
			name: "single_use_outside_decl_kept",
			input: []Finding{
				{File: "main.go", Line: 3, Severity: SeverityWarning, Category: CategoryMaintainability, Confidence: 0.9, Message: "`RateLimit` is unused."},
			},
			content: "package main\n\nvar RateLimit = 10\n\nfunc Report() int { return RateLimit }\n",
			keep:    true, // one occurrence outside the declaration: not proven used
		},
		{
			name: "used_twice_outside_decl_dropped",
			input: []Finding{
				{File: "main.go", Line: 11, Severity: SeverityWarning, Category: CategoryMaintainability, Confidence: 0.9, Message: "`RateLimit` is never used."},
			},
			content: existenceGoFixture,
			keep:    false, // two fmt.Println(RateLimit) uses outside the var line
		},

		// --- not exported ---
		{
			name: "exported_symbol_claimed_not_exported_dropped",
			input: []Finding{
				{File: "util.js", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is not exported."},
			},
			content: "export function compareTaskIds(a, b) {\n  return a === b;\n}\n",
			keep:    false,
		},
		{
			name: "named_export_list_claimed_not_exported_dropped",
			input: []Finding{
				{File: "util.js", Line: 3, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is not exported."},
			},
			content: "function compareTaskIds(a, b) { return a === b; }\nexport { compareTaskIds };\n",
			keep:    false,
		},
		{
			name: "go_exported_name_claimed_not_exported_dropped",
			input: []Finding{
				{File: "main.go", Line: 5, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`CompareTaskIds` is not exported."},
			},
			content: existenceGoFixture,
			keep:    false,
		},
		{
			name: "unexported_symbol_claimed_not_exported_kept",
			input: []Finding{
				{File: "util.js", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is not exported."},
			},
			content: existenceJSFixture,
			keep:    true, // no export statement in the file
		},
		{
			name: "cross_module_not_exported_kept",
			input: []Finding{
				{File: "a.go", Line: 1, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is not exported from `./utils`."},
			},
			content: "export function compareTaskIds(a, b) {\n  return a === b;\n}\n",
			keep:    true, // other modules are never read
		},

		// --- mixed claims ---
		{
			name: "mixed_claims_all_disproven_dropped",
			input: []Finding{
				{File: "a.go", Line: 5, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`fs` is unused and `compareTaskIds` is not defined."},
			},
			// Both identifiers must satisfy every present claim's proof: fs
			// imported and used twice; compareTaskIds declared and used twice.
			content: "import fs from \"node:fs\";\nfunction compareTaskIds(a, b) { return a === b; }\nfs.mkdirSync(\"x\");\nfs.rmSync(\"y\");\nconst t1 = compareTaskIds(\"a\", \"b\");\nconst t2 = compareTaskIds(\"c\", \"d\");\n",
			keep:    false,
		},
		{
			name: "mixed_claims_one_unproven_kept",
			input: []Finding{
				{File: "a.go", Line: 5, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`fs` is unused and `missingUtil` is not defined."},
			},
			content: existenceJSFixture,
			keep:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FilterExistenceClaims(tc.input, tc.content)
			if tc.input == nil {
				if len(got) != 0 {
					t.Fatalf("FilterExistenceClaims(nil) = %+v, want nil/empty", got)
				}
				return
			}
			if tc.keep && len(got) != len(tc.input) {
				t.Fatalf("finding should be kept; got %d of %d: %+v", len(got), len(tc.input), got)
			}
			if !tc.keep && len(got) != 0 {
				t.Fatalf("finding should be dropped; got %+v", got)
			}
		})
	}
}

// TestFilterExistenceClaims_preservesOrderAndInput asserts kept findings keep
// their order and the input slice is not modified.
func TestFilterExistenceClaims_preservesOrderAndInput(t *testing.T) {
	t.Parallel()
	input := []Finding{
		{File: "a.go", Line: 1, Severity: SeverityWarning, Category: CategoryBug, Confidence: 0.9, Message: "Unchecked error in `createRulesFixture`."},
		{File: "a.go", Line: 10, Severity: SeverityError, Category: CategoryCorrectness, Confidence: 0.95, Message: "`compareTaskIds` is not defined."},
		{File: "a.go", Line: 4, Severity: SeverityWarning, Category: CategoryStyle, Confidence: 0.9, Message: "`missingHelper` is not defined."},
	}
	inputCopy := append([]Finding(nil), input...)
	got := FilterExistenceClaims(input, existenceJSFixture)
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(got), got)
	}
	if got[0].Message != input[0].Message || got[1].Message != input[2].Message {
		t.Errorf("kept findings out of order: %+v", got)
	}
	for i := range input {
		if !reflect.DeepEqual(input[i], inputCopy[i]) {
			t.Errorf("input modified at %d: %+v", i, input[i])
		}
	}
}
