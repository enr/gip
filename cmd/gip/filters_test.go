package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupFilterRepos creates three clones of the same origin: "ahead" (one local
// commit), "stash" (in sync, clean tree, one stash entry) and "untracked" (in
// sync, one untracked file). It returns the config file listing them.
func setupFilterRepos(t *testing.T, root string) string {
	t.Helper()
	origin := filepath.Join(root, "origin.git")
	runGit(t, root, "init", "-q", "--bare", "-b", "main", origin)
	seed := filepath.Join(root, "seed")
	runGit(t, root, "clone", "-q", origin, seed)
	runGit(t, seed, "checkout", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "f"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", "f")
	runGit(t, seed, "commit", "-q", "-m", "c1")
	runGit(t, seed, "push", "-q", "origin", "main")

	var content string
	for _, name := range []string{"ahead", "stash", "untracked"} {
		dir := filepath.Join(root, name)
		runGit(t, root, "clone", "-q", origin, dir)
		content += fmt.Sprintf("- name: %s\n  local_path: %s\n  repository: \"https://example.com/%s.git\"\n", name, dir, name)
	}
	runGit(t, filepath.Join(root, "ahead"), "commit", "-q", "--allow-empty", "-m", "local")
	stash := filepath.Join(root, "stash")
	if err := os.WriteFile(filepath.Join(stash, "f"), []byte("2"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, stash, "stash", "-q")
	if err := os.WriteFile(filepath.Join(root, "untracked", "new"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := filepath.Join(root, ".gip")
	if err := os.WriteFile(cfg, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// runJSON runs gip --json with args and decodes the projects of the envelope.
func runJSON(t *testing.T, binPath, cfg string, args ...string) []map[string]interface{} {
	t.Helper()
	out, err := exec.Command(binPath, append([]string{"-f", cfg, "--json"}, args...)...).Output()
	if err != nil {
		t.Fatalf("gip %v failed: %v\n%s", args, err, out)
	}
	var env struct {
		Projects []map[string]interface{} `json:"projects"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("invalid JSON from gip %v: %v\n%s", args, err, out)
	}
	return env.Projects
}

func byName(projects []map[string]interface{}) map[string]map[string]interface{} {
	m := make(map[string]map[string]interface{}, len(projects))
	for _, p := range projects {
		m[p["name"].(string)] = p
	}
	return m
}

// TestBranchSyncFiltersUseOrLogic verifies that branch --unsynced matches a
// repo that is only ahead (not only diverged ones), as the other commands do.
func TestBranchSyncFiltersUseOrLogic(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	cfg := setupFilterRepos(t, root)
	binPath := buildBinary(t, root)

	got := byName(runJSON(t, binPath, cfg, "branch", "--unsynced"))
	if got["ahead"]["status"] != "ok" {
		t.Fatalf("branch --unsynced: expected ahead-only repo to match, got %v", got["ahead"])
	}
	if got["stash"]["status"] != "skipped" || got["stash"]["reason"] != "not ahead or behind" {
		t.Fatalf("branch --unsynced: expected in-sync repo skipped, got %v", got["stash"])
	}

	got = byName(runJSON(t, binPath, cfg, "branch", "--behind"))
	if got["ahead"]["status"] != "skipped" || got["ahead"]["reason"] != "not behind" {
		t.Fatalf("branch --behind: expected ahead-only repo skipped as not behind, got %v", got["ahead"])
	}
	got = byName(runJSON(t, binPath, cfg, "branch", "--ahead"))
	if got["ahead"]["status"] != "ok" {
		t.Fatalf("branch --ahead: expected ahead-only repo to match, got %v", got["ahead"])
	}
}

// TestListDirtyIgnoresStashOnly verifies that list --dirty uses the same notion
// of "dirty" as the other commands: a stash alone does not count.
func TestListDirtyIgnoresStashOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	cfg := setupFilterRepos(t, root)
	binPath := buildBinary(t, root)

	got := byName(runJSON(t, binPath, cfg, "list", "--dirty"))
	if _, ok := got["stash"]; ok {
		t.Fatalf("list --dirty: stash-only repo must not be listed, got %v", got)
	}
	if _, ok := got["untracked"]; !ok {
		t.Fatalf("list --dirty: repo with untracked files must be listed, got %v", got)
	}
	// Consistency with branch --dirty.
	got = byName(runJSON(t, binPath, cfg, "branch", "--dirty"))
	if got["stash"]["status"] != "skipped" || got["untracked"]["status"] != "ok" {
		t.Fatalf("branch --dirty: unexpected results %v", got)
	}
}
