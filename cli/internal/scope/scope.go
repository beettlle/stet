// Package scope partitions diff hunks into to-review (sent to the LLM) and
// approved (already reviewed at last_reviewed_at, unchanged by strict or
// semantic match). Used for incremental review in Phase 2.3.
package scope

import (
	"context"

	"stet/cli/internal/diff"
	"stet/cli/internal/erruser"
	"stet/cli/internal/hunkid"
)

// Result holds the partition of current hunks into to-review and approved.
type Result struct {
	// ToReview are hunks to send to the LLM (new or changed since last_reviewed_at).
	ToReview []diff.Hunk
	// Approved are hunks that existed at last_reviewed_at and are unchanged
	// (strict match) or same semantically (comment/whitespace only).
	Approved []diff.Hunk
}

// Partition splits hunks for baseline..head into ToReview and Approved using
// the "already reviewed" set from baseline..lastReviewedAt. When
// lastReviewedAt is empty (first run), all current hunks go to ToReview and
// Approved is nil. Context is used for cancellation when running git diff.
func Partition(ctx context.Context, repoRoot, baselineRef, headRef, lastReviewedAt string, opts *diff.Options) (Result, error) {
	current, err := diff.Hunks(ctx, repoRoot, baselineRef, headRef, opts)
	if err != nil {
		return Result{}, erruser.New("Could not compute diff.", err)
	}
	if len(current) == 0 {
		return Result{ToReview: nil, Approved: nil}, nil
	}
	if lastReviewedAt == "" {
		return Result{ToReview: current, Approved: nil}, nil
	}

	reviewed, err := diff.Hunks(ctx, repoRoot, baselineRef, lastReviewedAt, opts)
	if err != nil {
		return Result{}, erruser.New("Could not compute diff.", err)
	}

	strictIDs := make(map[string]struct{}, len(reviewed))
	semanticIDs := make(map[string]struct{}, len(reviewed))
	for _, h := range reviewed {
		strictIDs[hunkid.StrictHunkID(h.FilePath, h.RawContent)] = struct{}{}
		semanticIDs[hunkid.SemanticHunkID(h.FilePath, h.RawContent)] = struct{}{}
	}

	toReview := make([]diff.Hunk, 0, len(current))
	approved := make([]diff.Hunk, 0, len(current))
	for _, h := range current {
		sid := hunkid.StrictHunkID(h.FilePath, h.RawContent)
		mid := hunkid.SemanticHunkID(h.FilePath, h.RawContent)
		if _, ok := strictIDs[sid]; ok {
			approved = append(approved, h)
			continue
		}
		if _, ok := semanticIDs[mid]; ok {
			approved = append(approved, h)
			continue
		}
		toReview = append(toReview, h)
	}
	return Result{ToReview: toReview, Approved: approved}, nil
}
