package run

import (
	"fmt"
	"io"
	"os"
	"sort"

	"stet/cli/internal/diff"
	"stet/cli/internal/rules"
)

// loadRulesForHunks loads the Cursor rules for each distinct file in hunks
// through a loader backed by the review digest cache in stateDir, then saves
// the cache. When warnDropped is true (default rule assembly, not the
// selector), it writes one line to warnOut (nil = os.Stderr) if the rule
// budget drops any digest section for any file. Cache save failures are
// reported on warnOut and never fail the review.
func loadRulesForHunks(repoRoot, stateDir string, hunks []diff.Hunk, ruleBudget int, warnDropped bool, warnOut io.Writer) map[string][]rules.CursorRule {
	if warnOut == nil {
		warnOut = os.Stderr
	}
	loader := rules.NewLoader(repoRoot).WithDigestCache(rules.OpenDigestCache(stateDir))
	byFile := make(map[string][]rules.CursorRule)
	for _, h := range hunks {
		if _, ok := byFile[h.FilePath]; !ok {
			byFile[h.FilePath] = loader.RulesForFile(h.FilePath)
		}
	}
	if err := loader.SaveDigests(); err != nil {
		fmt.Fprintf(warnOut, "stet: rules digest cache not saved: %v\n", err)
	}
	if warnDropped {
		warnDroppedRuleSections(warnOut, byFile, ruleBudget)
	}
	return byFile
}

// warnDroppedRuleSections writes one warning naming how many distinct rule
// digest sections the budget drops across all files, with the first few labels.
func warnDroppedRuleSections(out io.Writer, byFile map[string][]rules.CursorRule, ruleBudget int) {
	const maxNamed = 3
	dropped := make(map[string]struct{})
	for file, ruleList := range byFile {
		for _, label := range rules.BuildReviewCriteria(ruleList, file, ruleBudget).Dropped {
			dropped[label] = struct{}{}
		}
	}
	if len(dropped) == 0 {
		return
	}
	labels := make([]string, 0, len(dropped))
	for label := range dropped {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	named := labels[:min(len(labels), maxNamed)]
	fmt.Fprintf(out, "stet: rule budget of %d tokens dropped %d rule section(s) from review prompts (e.g. %q); raise rules_max_tokens to include more\n", ruleBudget, len(labels), named)
}
