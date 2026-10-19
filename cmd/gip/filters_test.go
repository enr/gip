package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enr/gip/lib/core"
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

func TestGitStateFilterSkipReason(t *testing.T) {
	clean := repoState{}
	stashOnly := repoState{dirty: core.DirtyStatus{Stashed: true}}
	untracked := repoState{dirty: core.DirtyStatus{Untracked: true}}
	ahead := repoState{sync: core.BranchSyncStatus{Ahead: 1}}
	behind := repoState{sync: core.BranchSyncStatus{Behind: 2}}
	diverged := repoState{sync: core.BranchSyncStatus{Ahead: 1, Behind: 2}}
	noRemote := repoState{sync: core.BranchSyncStatus{NoRemote: true}}
	unsynced := gitStateFilter{behind: true, ahead: true}

	cases := []struct {
		name   string
		filter gitStateFilter
		state  repoState
		want   string
	}{
		{"no filter", gitStateFilter{}, clean, ""},
		{"dirty: stash alone", gitStateFilter{dirty: true}, stashOnly, "not dirty"},
		{"dirty: untracked", gitStateFilter{dirty: true}, untracked, ""},
		{"behind: ahead only", gitStateFilter{behind: true}, ahead, "not behind"},
		{"ahead: behind only", gitStateFilter{ahead: true}, behind, "not ahead"},
		{"unsynced: ahead only", unsynced, ahead, ""},
		{"unsynced: behind only", unsynced, behind, ""},
		{"unsynced: diverged", unsynced, diverged, ""},
		{"unsynced: in sync", unsynced, clean, "not ahead or behind"},
		{"unsynced: no upstream", unsynced, noRemote, "not ahead or behind"},
		{"dirty and behind: clean", gitStateFilter{dirty: true, behind: true}, behind, "not dirty"},
	}
	for _, tc := range cases {
		if got := tc.filter.skipReason(tc.state); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestListReportsUnreadableRepo verifies that list reports a repository whose
// state cannot be read instead of showing it as if nothing was wrong.
func TestListReportsUnreadableRepo(t *testing.T) {
	root := t.TempDir()
	binPath := buildBinary(t, root)
	// An empty .git directory: isProjectDir accepts it, git status fails.
	broken := filepath.Join(root, "broken")
	makeGitRepo(t, broken)
	cfg := filepath.Join(root, ".gip")
	makeConfig(t, cfg, []string{broken})

	cmd := exec.Command(binPath, "-f", cfg, "--json", "list")
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+root)
	out, err := cmd.Output()
	if err == nil {
		t.Fatalf("expected non-zero exit for an unreadable repository:\n%s", out)
	}
	var env listOutputJSON
	if jerr := json.Unmarshal(out, &env); jerr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jerr, out)
	}
	if len(env.Projects) != 1 || env.Projects[0].Error == "" {
		t.Fatalf("expected the error in the project entry, got %+v", env.Projects)
	}
}

// TestListReportsTimeout verifies that a repository whose git status outlives
// --timeout is shown as TIMEOUT, with an error naming the timeout rather than
// the "signal: killed" of the stopped git process.
func TestListReportsTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake git is a shell script")
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	binPath := buildBinary(t, root)
	repo := filepath.Join(root, "slow")
	runGit(t, root, "init", "-q", repo)
	cfg := filepath.Join(root, ".gip")
	makeConfig(t, cfg, []string{repo})
	// A git that hangs on status and behaves normally otherwise.
	fakeDir := filepath.Join(root, "fakebin")
	if err := os.MkdirAll(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in status) exec sleep 30;; esac\nexec %q \"$@\"\n", realGit)
	if err := os.WriteFile(filepath.Join(fakeDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "-f", cfg, "list", "--timeout", "1")
	cmd.Env = append(os.Environ(), "PATH="+fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected non-zero exit for a timed out repository:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "TIMEOUT") {
		t.Errorf("expected TIMEOUT in the table, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "timed out after 1s") {
		t.Errorf("expected the timeout in the error, got:\n%s", stderr.String())
	}
}
