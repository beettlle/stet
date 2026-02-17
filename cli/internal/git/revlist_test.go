package git

import (
	"testing"
)

func TestRevList_twoCommitsInRange(t *testing.T) {
	t.Parallel()
	repo := initRepo(t)
	// Add third commit so HEAD~2..HEAD yields 2 SHAs.
	writeFile(t, repo, "f3.txt", "c\n")
	run(t, repo, "git", "add", "f3.txt")
	run(t, repo, "git", "commit", "-m", "c3")
	shas, err := RevList(repo, "HEAD~2", "HEAD")
	if err != nil {
		t.Fatalf("RevList HEAD~2..HEAD: %v", err)
	}
	if len(shas) != 2 {
		t.Errorf("RevList HEAD~2..HEAD: got %d SHAs, want 2", len(shas))
	}
	for i, s := range shas {
		if len(s) != 40 {
			t.Errorf("RevList SHA[%d] = %q, want 40-char hex", i, s)
		}
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
