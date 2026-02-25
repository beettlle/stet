package git

import (
	"testing"
)

func TestRevList_twoCommitsInRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	// initRepo creates 2 commits (c1, c2). Record SHA before adding c3.
	shaBeforeC3 := runOut(t, repo, "git", "rev-parse", "HEAD")

	writeFile(t, repo, "f3.txt", "c\n")
	run(t, repo, "git", "add", "f3.txt")
	run(t, repo, "git", "commit", "-m", "c3")

	shaAfterC3 := runOut(t, repo, "git", "rev-parse", "HEAD")
	if shaBeforeC3 == shaAfterC3 {
		t.Fatal("commit c3 did not change HEAD; test setup broken")
	}

	// HEAD~2..HEAD spans c2 and c3 (the two most recent commits).
	baselineSHA := runOut(t, repo, "git", "rev-parse", "HEAD~2")
	shas, err := RevList(repo, baselineSHA, "HEAD")
	if err != nil {
		t.Fatalf("RevList %s..HEAD: %v", baselineSHA[:8], err)
	}
	if len(shas) != 2 {
		t.Fatalf("RevList: got %d SHAs, want 2", len(shas))
	}
	for i, s := range shas {
		if len(s) != 40 {
			t.Errorf("RevList SHA[%d] = %q, want 40-char hex", i, s)
		}
	}
	// rev-list returns newest first; verify the returned SHAs match c3 and c2.
	if shas[0] != shaAfterC3 {
		t.Errorf("SHA[0] = %s, want c3 SHA %s", shas[0], shaAfterC3)
	}
	if shas[1] != shaBeforeC3 {
		t.Errorf("SHA[1] = %s, want c2 SHA %s", shas[1], shaBeforeC3)
	}
}

func TestRevList_oneCommitInRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	shas, err := RevList(repo, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("RevList HEAD~1..HEAD: %v", err)
	}
	if len(shas) != 1 {
		t.Errorf("RevList HEAD~1..HEAD: got %d SHAs, want 1", len(shas))
	}
}

func TestRevList_emptyRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	shas, err := RevList(repo, "HEAD", "HEAD")
	if err != nil {
		t.Fatalf("RevList HEAD..HEAD: %v", err)
	}
	if shas != nil {
		t.Errorf("RevList HEAD..HEAD: got %v, want nil", shas)
	}
}

func TestRevList_invalidSince(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	_, err := RevList(repo, "not-a-ref", "HEAD")
	if err == nil {
		t.Fatal("RevList(invalid since): expected error")
	}
}

func TestRevList_invalidUntil(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	_, err := RevList(repo, "HEAD", "not-a-ref")
	if err == nil {
		t.Fatal("RevList(invalid until): expected error")
	}
}

func TestRevList_emptyRepoRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := RevList(dir, "HEAD", "HEAD")
	if err == nil {
		t.Fatal("RevList(not a repo): expected error")
	}
}

func TestRevList_emptyRefs(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	_, err := RevList(repo, "", "HEAD")
	if err == nil {
		t.Fatal("RevList(since empty): expected error")
	}
	_, err = RevList(repo, "HEAD", "")
	if err == nil {
		t.Fatal("RevList(until empty): expected error")
	}
}
