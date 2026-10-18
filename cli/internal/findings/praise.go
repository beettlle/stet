// Package findings defines the schema for code review findings. This file
// implements the praise-only filter: low-severity findings that only praise
// the change or confirm it is correct, without a required action, are dropped.

package findings

import (
	"regexp"
	"strings"
	"sync"
)

// praisePatterns are case-insensitive regular expressions that indicate a
// finding message only praises or confirms the change. Word boundaries keep
// "incorrectly" from matching "correctly".
var praisePatterns = []string{
	`\bcorrectly\b`,
	`\bproperly handles\b`,
	`\bgood\b`,
	`\bwell done\b`,
	`\bnice\b`,
	`\blooks good\b`,
	`\bis correct\b`,
}

// praiseCaveatPatterns mark a message as describing a defect or caveat even
// when it also contains praise wording (e.g. "correctly parses X, but Y can
// panic" or "does not correctly handle nil"). Such findings are kept.
var praiseCaveatPatterns = []string{
	`\bnot\b`,
	`n't\b`,
	`\bnever\b`,
	`\bbut\b`,
	`\bhowever\b`,
	`\bexcept\b`,
	`\bmissing\b`,
	`\bfails?\b`,
}

var (
	praiseCompiledOnce sync.Once
	praiseCompiled     *regexp.Regexp
	caveatCompiled     *regexp.Regexp
)

func initPraiseCompiled() {
	praiseCompiledOnce.Do(func() {
		praiseCompiled = regexp.MustCompile("(?i)(?:" + strings.Join(praisePatterns, "|") + ")")
		caveatCompiled = regexp.MustCompile("(?i)(?:" + strings.Join(praiseCaveatPatterns, "|") + ")")
	})
}

// FilterPraiseOnly drops findings that only praise the change: severity is
// info or nitpick, Suggestion is empty after trimming, and Message matches a
// praise pattern without a caveat/defect pattern. Warning and error findings
// are always kept. Returns a new slice; the input is not modified. Order of
// kept findings is preserved.
func FilterPraiseOnly(list []Finding) []Finding {
	if len(list) == 0 {
		return nil
	}
	initPraiseCompiled()
	out := make([]Finding, 0, len(list))
	for _, f := range list {
		if isPraiseOnly(f) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// isPraiseOnly reports whether f is a low-severity, suggestion-less finding
// whose message is praise. Caller must have run initPraiseCompiled().
func isPraiseOnly(f Finding) bool {
	if f.Severity != SeverityInfo && f.Severity != SeverityNitpick {
		return false
	}
	if strings.TrimSpace(f.Suggestion) != "" {
		return false
	}
	return praiseCompiled.MatchString(f.Message) && !caveatCompiled.MatchString(f.Message)
}
