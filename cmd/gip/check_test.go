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

func TestSameRepository(t *testing.T) {
	same := [][2]string{
		{"https://github.com/enr/gip.git", "git@github.com:enr/gip.git"},
		{"https://github.com/enr/gip", "ssh://git@github.com:22/enr/gip.git"},
		{"https://GitHub.com/Enr/Gip/", "https://github.com/enr/gip"},
		{"git@gitlab.com:group/sub/proj.git", "https://user@gitlab.com/group/sub/proj"},
		{"/srv/git/a.git", "file:///srv/git/a.git"},
		{"/srv/git/a.git", "/srv/git/a"},
	}
	for _, p := range same {
		if !sameRepository(p[0], p[1]) {
			t.Errorf("expected %q and %q to match", p[0], p[1])
		}
	}
	different := [][2]string{
		{"https://github.com/enr/gip.git", "https://github.com/enr/gip2.git"},
		{"https://github.com/enr/gip.git", "https://gitlab.com/enr/gip.git"},
		{"git@github.com:enr/gip.git", "git@github.com:other/gip.git"},
		{"/srv/git/A.git", "/srv/git/a.git"},
	}
	for _, p := range different {
		if sameRepository(p[0], p[1]) {
			t.Errorf("expected %q and %q not to match", p[0], p[1])
		}
	}
}

// checkFixture is a repository tracking origin.git, a second clone of it and
// a plain directory, plus the gip binary.
type checkFixture struct {
	root, repo, clone, origin, notGit, bin, cfg string
}

func newCheckFixture(t *testing.T) checkFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	f := checkFixture{root: root, repo: setupBranchRepo(t, root), origin: filepath.Join(root, "origin.git"),
		clone: filepath.Join(root, "clone"), notGit: filepath.Join(root, "plain"), cfg: filepath.Join(root, ".gip")}
	runGit(t, root, "clone", "-q", f.origin, f.clone)
	if err := os.MkdirAll(f.notGit, 0755); err != nil {
		t.Fatal(err)
	}
	f.bin = buildBinary(t, root)
	return f
}

func (f checkFixture) writeConfig(t *testing.T, entries ...string) {
	t.Helper()
	if err := os.WriteFile(f.cfg, []byte(strings.Join(entries, "")), 0644); err != nil {
		t.Fatal(err)
	}
}

func checkConfigEntry(name, path, repository, extra string) string {
	return fmt.Sprintf("- name: %s\n  local_path: %s\n  repository: %q\n%s", name, path, repository, extra)
}

// assertCheckIssue verifies the status of a project and, when message is not
// empty, that one of its issues contains it.
func assertCheckIssue(t *testing.T, p checkProjectJSON, status, message string) {
	t.Helper()
	if p.Status != status {
		t.Errorf("%s: status %q, want %q (issues %+v)", p.Name, p.Status, status, p.Issues)
	}
	if message == "" {
		return
	}
	for _, i := range p.Issues {
		if strings.Contains(i.Message, message) {
			return
		}
	}
	t.Errorf("%s: no issue containing %q in %+v", p.Name, message, p.Issues)
}

func TestCheckCommandJSON(t *testing.T) {
	f := newCheckFixture(t)
	root := f.root
	f.writeConfig(t,
		checkConfigEntry("good", f.repo, f.origin, "  branches: [rel]\n"),
		checkConfigEntry("nobranch", f.clone, "file://"+f.origin, "  branches: [nope]\n"),
		checkConfigEntry("wrongremote", filepath.Join(root, "work"), "https://example.com/other.git", ""),
		checkConfigEntry("missing", filepath.Join(root, "missing"), f.origin, ""),
		checkConfigEntry("gone", filepath.Join(root, "gone"), filepath.Join(root, "gone.git"), ""),
		checkConfigEntry("notgit", f.notGit, f.origin, ""),
		checkConfigEntry("dup1", filepath.Join(root, "dup"), f.origin, ""),
		checkConfigEntry("dup2", filepath.Join(root, "dup"), f.origin, ""),
		checkConfigEntry("off", f.notGit, f.origin, "  disabled: true\n"))

	out, err := exec.Command(f.bin, "-f", f.cfg, "--json", "check", "--remote").Output()
	if err == nil {
		t.Fatalf("expected exit status 1 with errors:\n%s", out)
	}
	var res checkOutputJSON
	if jerr := json.Unmarshal(out, &res); jerr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jerr, out)
	}
	got := make(map[string]checkProjectJSON)
	for _, p := range res.Projects {
		got[p.Name] = p
	}
	if _, ok := got["off"]; ok {
		t.Errorf("disabled project must not be checked")
	}
	want := map[string][2]string{
		"good":        {"ok", ""},
		"nobranch":    {"warning", "branch nope (in branches) not found locally"},
		"wrongremote": {"error", "no remote points to repository https://example.com/other.git"},
		"missing":     {"warning", "directory missing"},
		"gone":        {"error", "not reachable"},
		"notgit":      {"error", "not a git repository"},
		"dup1":        {"error", "local_path shared with dup2"},
		"dup2":        {"error", "local_path shared with dup1"},
	}
	for name, w := range want {
		if p, ok := got[name]; ok {
			assertCheckIssue(t, p, w[0], w[1])
		} else {
			t.Errorf("%s: not reported", name)
		}
	}
	if res.Summary.Total != len(want) || res.Summary.OK != 1 || res.Summary.Warnings != 2 || res.Summary.Errors != 5 {
		t.Errorf("unexpected summary %+v", res.Summary)
	}
}

func TestCheckCommandText(t *testing.T) {
	f := newCheckFixture(t)
	f.writeConfig(t,
		checkConfigEntry("good", f.repo, f.origin, "  tags: [ok]\n"),
		checkConfigEntry("notgit", f.notGit, f.origin, ""))

	out, err := exec.Command(f.bin, "-f", f.cfg, "check", "--tag", "ok").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "OK: 1   Errors: 0   Warnings: 0") {
		t.Fatalf("expected a clean check: %v\n%s", err, out)
	}
	out, err = exec.Command(f.bin, "-f", f.cfg, "check").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "[ERR]  local_path is not a git repository") {
		t.Fatalf("expected exit status 1 and the error in the text report: %v\n%s", err, out)
	}
}

func TestIsAuthError(t *testing.T) {
	auth := []string{
		"exit status 128: fatal: could not read Password for 'https://u@github.com': terminal prompts disabled",
		"exit status 128: git@github.com: Permission denied (publickey).",
		"exit status 128: remote: HTTP Basic: Access denied\nfatal: Authentication failed for 'https://gitlab.com/x.git/'",
	}
	for _, m := range auth {
		if !isAuthError(fmt.Errorf("%s", m)) {
			t.Errorf("expected an authentication error: %q", m)
		}
	}
	if isAuthError(fmt.Errorf("exit status 128: fatal: repository 'https://github.com/enr/nope.git/' not found")) {
		t.Error("a missing repository is not an authentication error")
	}
}
