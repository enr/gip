package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var releaseRepoNames = []string{"ahead", "ontag", "notag", "empty"}

// setupReleaseRepos creates four repos: "ahead" (2 commits after an annotated
// tag with dashes in its name), "ontag" (HEAD on a lightweight tag), "notag"
// (1 commit, never tagged) and "empty" (no commits); it returns their paths
// and a gip config listing them.
func setupReleaseRepos(t *testing.T, root string) (map[string]string, string) {
	t.Helper()
	repos := map[string]string{}
	var content strings.Builder
	for _, name := range releaseRepoNames {
		repos[name] = filepath.Join(root, name)
		runGit(t, root, "init", "-q", repos[name])
		fmt.Fprintf(&content, "- name: %s\n  local_path: %s\n  repository: \"https://example.com/%s.git\"\n", name, repos[name], name)
	}
	for _, name := range []string{"ahead", "ontag", "notag"} {
		runGit(t, repos[name], "commit", "-q", "--allow-empty", "-m", "c1")
	}
	runGit(t, repos["ahead"], "tag", "-a", "v1.0-rc-1", "-m", "rc")
	runGit(t, repos["ahead"], "commit", "-q", "--allow-empty", "-m", "c2")
	runGit(t, repos["ahead"], "commit", "-q", "--allow-empty", "-m", "c3")
	runGit(t, repos["ontag"], "tag", "v2.0.0")
	cfg := filepath.Join(root, ".gip")
	if err := os.WriteFile(cfg, []byte(content.String()), 0644); err != nil {
		t.Fatal(err)
	}
	return repos, cfg
}

func runReleases(t *testing.T, binPath, cfg string, args ...string) string {
	t.Helper()
	out, err := exec.Command(binPath, append([]string{"-f", cfg}, args...)...).Output()
	if err != nil {
		t.Fatalf("gip %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

type releaseProjectJSON struct {
	Name            string `json:"name"`
	Status          string `json:"status"`
	LastTag         string `json:"last_tag"`
	CommitsSinceTag *int   `json:"commits_since_tag"`
	Unreleased      bool   `json:"unreleased"`
}

func checkReleaseJSON(t *testing.T, p releaseProjectJSON) {
	t.Helper()
	wants := map[string]releaseProjectJSON{
		"ahead": {LastTag: "v1.0-rc-1", CommitsSinceTag: intPtr(2), Unreleased: true},
		"ontag": {LastTag: "v2.0.0", CommitsSinceTag: intPtr(0)},
		"notag": {CommitsSinceTag: intPtr(1), Unreleased: true},
		"empty": {},
	}
	w := wants[p.Name]
	if p.Status != "ok" || p.LastTag != w.LastTag || p.Unreleased != w.Unreleased ||
		fmt.Sprint(derefInt(p.CommitsSinceTag)) != fmt.Sprint(derefInt(w.CommitsSinceTag)) {
		t.Errorf("%s: got %+v (commits %v), want %+v (commits %v)", p.Name, p, derefInt(p.CommitsSinceTag), w, derefInt(w.CommitsSinceTag))
	}
}

func intPtr(n int) *int { return &n }

// derefInt renders a *int as its value, or "nil".
func derefInt(p *int) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprint(*p)
}

func TestReleases(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	repos, cfg := setupReleaseRepos(t, root)
	binPath := buildBinary(t, root)

	t.Run("json", func(t *testing.T) {
		var parsed struct {
			Projects []releaseProjectJSON `json:"projects"`
		}
		out := runReleases(t, binPath, cfg, "--json", "releases")
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if len(parsed.Projects) != len(releaseRepoNames) {
			t.Fatalf("expected %d projects:\n%s", len(releaseRepoNames), out)
		}
		for _, p := range parsed.Projects {
			checkReleaseJSON(t, p)
		}
	})

	t.Run("pending", func(t *testing.T) {
		out := runReleases(t, binPath, cfg, "-q", "releases", "--pending")
		for name, listed := range map[string]bool{"ahead": true, "notag": true, "ontag": false, "empty": false} {
			if strings.Contains(out, repos[name]) != listed {
				t.Errorf("--pending: %s listed=%v, want %v:\n%s", name, !listed, listed, out)
			}
		}
	})

	t.Run("match", func(t *testing.T) {
		out := runReleases(t, binPath, cfg, "-q", "releases", "--match", "v2*")
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "ahead ") && !strings.Contains(l, "(none)") {
				t.Errorf("--match v2*: ahead must have no matching tag: %q", l)
			}
		}
	})
}
