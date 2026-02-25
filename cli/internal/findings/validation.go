package findings

import (
	"errors"
	"fmt"
)

// Valid severities and categories for Validate.
var (
	validSeverities = map[Severity]struct{}{
		SeverityError: {}, SeverityWarning: {}, SeverityInfo: {}, SeverityNitpick: {},
	}
	validCategories = map[Category]struct{}{
		CategoryBug: {}, CategorySecurity: {}, CategoryCorrectness: {}, CategoryPerformance: {},
		CategoryStyle: {}, CategoryMaintainability: {}, CategoryBestPractice: {}, CategoryTesting: {},
		CategoryDocumentation: {}, CategoryDesign: {}, CategoryAccessibility: {},
	}
)

// Normalize mutates the finding in place: invalid severity is set to SeverityWarning,
// invalid category is set to CategoryBug. Call Validate after Normalize to check
// other constraints (message, file, confidence, range). After Normalize, Validate
// will accept severity and category because those defaults are in validSeverities
// and validCategories.
func (f *Finding) Normalize() {
	if f == nil {
		return
	}
	if _, ok := validSeverities[f.Severity]; !ok {
		f.Severity = SeverityWarning
	}
	if _, ok := validCategories[f.Category]; !ok {
		f.Category = CategoryBug
	}
}

// Validate checks that the finding has required fields and allowed enum values.
// Line and range are optional (file-only findings are valid). It returns an error
// if: severity or category is missing or not in the allowed set; confidence is
// not in [0, 1]; file or message is empty; or range is present but invalid (start > end).
func (f *Finding) Validate() error {
	if f == nil {
		return errors.New("finding is nil")
	}
	if f.Severity == "" {
		return errors.New("severity is required")
	}
	if _, ok := validSeverities[f.Severity]; !ok {
		return fmt.Errorf("invalid severity %q", f.Severity)
	}
	if f.Category == "" {
		return errors.New("category is required")
	}
	if _, ok := validCategories[f.Category]; !ok {
		return fmt.Errorf("invalid category %q", f.Category)
	}
	if f.Confidence < 0 || f.Confidence > 1 {
		return fmt.Errorf("confidence %g must be between 0 and 1", f.Confidence)
	}
	if f.File == "" {
		return errors.New("file is required")
	}
	if f.Message == "" {
		return errors.New("message is required")
	}
	if f.Range != nil && f.Range.Start > f.Range.End {
		return fmt.Errorf("range start %d must be <= end %d", f.Range.Start, f.Range.End)
	}
	return nil
}
