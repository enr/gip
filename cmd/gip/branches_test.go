package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// setupBranchRepo creates a clone whose branches rel (behind), dev (ahead) and
// main (checked out) track an origin, and returns its path.
func setupBranchRepo(t *testing.T, root string) string {
	t.Helper()
	origin := filepath.Join(root, "origin.git")
	runGit(t, root, "init", "-q", "--bare", "-b", "main", origin)
	work := filepath.Join(root, "work")
	runGit(t, root, "clone", "-q", origin, work)
	runGit(t, work, "checkout", "-q", "-b", "main")
	runGit(t, work, "commit", "-q", "--allow-empty", "-m", "c1")
	runGit(t, work, "push", "-q", "origin", "main")
	for _, b := range []string{"rel", "dev"} {
		runGit(t, work, "checkout", "-q", "-b", b, "main")
		runGit(t, work, "push", "-q", "origin", b)
	}
	repo := filepath.Join(root, "repo")
	runGit(t, root, "clone", "-q", origin, repo)
	runGit(t, repo, "branch", "rel", "origin/rel")
	runGit(t, repo, "branch", "dev", "origin/dev")
	// origin moves rel forward; local dev gets a commit of its own.
	runGit(t, work, "checkout", "-q", "rel")
	runGit(t, work, "commit", "-q", "--allow-empty", "-m", "r2")
	runGit(t, work, "push", "-q", "origin", "rel")
	runGit(t, repo, "checkout", "-q", "dev")
	runGit(t, repo, "commit", "-q", "--allow-empty", "-m", "local")
	runGit(t, repo, "checkout", "-q", "main")
	runGit(t, repo, "fetch", "-q")
	return repo
}

func TestBranchesStatusAndPull(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	repo := setupBranchRepo(t, root)
	binPath := buildBinary(t, root)
	cfg := filepath.Join(root, ".gip")
	content := fmt.Sprintf("- name: with\n  local_path: %s\n  repository: \"https://example.com/a.git\"\n  branches: [rel, dev, nope]\n"+
		"- name: without\n  local_path: %s\n  repository: \"https://example.com/b.git\"\n", repo, repo)
	if err := os.WriteFile(cfg, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(binPath, "-f", cfg, "status").CombinedOutput()
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "[rel]  ↓1") || !strings.Contains(string(out), "[dev]  ↑1") {
		t.Fatalf("expected per-branch sync lines:\n%s", out)
	}

	// -j 1: both projects share the same repo, and concurrent "git pull" runs
	// race on FETCH_HEAD (fatal with pull.rebase=true: "Cannot rebase onto
	// multiple branches").
	out, err = exec.Command(binPath, "-f", cfg, "pull", "-j", "1").CombinedOutput()
	if err != nil {
		t.Fatalf("pull failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Errors: 0") {
		t.Fatalf("expected no errors (ahead-only branch must be skipped, not failed):\n%s", out)
	}
	if got := runGit(t, repo, "rev-parse", "rel"); got != runGit(t, repo, "rev-parse", "origin/rel") {
		t.Fatalf("rel was not fast-forwarded")
	}
	if cur := strings.TrimSpace(runGit(t, repo, "branch", "--show-current")); cur != "main" {
		t.Fatalf("current branch changed to %q", cur)
	}
}

func TestBranchesAbsentKeepsBehaviour(t *testing.T) {
	p := gipProject{}
	if got := p.extraBranches("main"); len(got) != 0 {
		t.Fatalf("expected no extra branches, got %v", got)
	}
	p = gipProject{Branches: []string{"main", " rel ", "rel", "", "-x", "dev"}}
	got := p.extraBranches("main")
	if strings.Join(got, ",") != "rel,dev" {
		t.Fatalf("got %v", got)
	}
}

// The --behind/--ahead filters must be evaluated per extra branch, not on the
// checked-out branch (which pull has just updated, or which is in sync).
func TestBranchesPullBehindFilter(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	repo := setupBranchRepo(t, root)
	// hot tracks origin/rel: the upstream name differs from the local one.
	runGit(t, repo, "branch", "hot", "rel")
	runGit(t, repo, "branch", "-q", "-u", "origin/rel", "hot")
	binPath := buildBinary(t, root)
	cfg := filepath.Join(root, ".gip")
	content := fmt.Sprintf("- name: with\n  local_path: %s\n  repository: \"https://example.com/a.git\"\n  branches: [rel, dev, hot]\n", repo)
	if err := os.WriteFile(cfg, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(binPath, "-f", cfg, "pull", "--behind").CombinedOutput()
	if err != nil {
		t.Fatalf("pull --behind failed: %v\n%s", err, out)
	}
	if got := runGit(t, repo, "rev-parse", "rel"); got != runGit(t, repo, "rev-parse", "origin/rel") {
		t.Fatalf("rel (behind) was not fast-forwarded by pull --behind:\n%s", out)
	}
	if got := runGit(t, repo, "rev-parse", "hot"); got != runGit(t, repo, "rev-parse", "origin/rel") {
		t.Fatalf("hot was not fast-forwarded from its upstream origin/rel:\n%s", out)
	}
}
