package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestErrorHint(t *testing.T) {
	cases := map[string]string{
		"fatal: could not read Username for 'https://github.com': terminal prompts disabled": "authentication required",
		"git@github.com: Permission denied (publickey).":                                     "SSH key rejected",
		"Host key verification failed.":                                                      "unknown SSH host key",
		"fatal: unable to access 'https://x/': Could not resolve host: x":                    "host not resolved",
		"ssh: connect to host x port 22: Connection timed out":                               "server not reachable",
		"fatal: unable to access 'https://x/': SSL certificate problem: self-signed":         "TLS certificate",
		"remote: Repository not found.\nfatal: repository 'https://x/y.git/' not found":      "repository not found",
		"fatal: repository 'x' does not exist":                                               "repository not found",
		"fatal: destination path '/a' already exists and is not an empty directory.":         "local_path is not empty",
		"fatal: not a git repository (or any of the parent directories): .git":               "not a git repository",
		"fatal: Unable to create '/r/.git/index.lock': File exists.":                         "another git process",
		"There is no tracking information for the current branch.":                           "no upstream",
		"You are not currently on a branch.":                                                 "detached HEAD",
		"error: Your local changes to the following files would be overwritten by merge":     "commit or stash",
		"fatal: Not possible to fast-forward, aborting.":                                     "branches diverged",
		"fatal: couldn't find remote ref main":                                               "no longer exists on the remote",
		`exec: "nosuchcmd": executable file not found in $PATH`:                              "command not found",
		"exit status 1": "",
	}
	for msg, want := range cases {
		got := errorHint(msg)
		if want == "" && got != "" || !strings.Contains(got, want) {
			t.Errorf("errorHint(%q) = %q, want it to mention %q", msg, got, want)
		}
	}
}

// TestActionableErrors checks that the errors a user meets first say what to
// do: unexpected arguments, git missing from PATH, missing directories.
func TestActionableErrors(t *testing.T) {
	root := t.TempDir()
	binPath := buildBinary(t, root)
	t.Setenv("GIP_FILE", "")

	notGit := filepath.Join(root, "notgit")
	if err := os.MkdirAll(notGit, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(root, "gip.yml")
	content := "- name: gone\n  local_path: " + filepath.Join(root, "gone") + "\n  repository: x\n" +
		"- name: notgit\n  local_path: " + notGit + "\n  repository: x\n"
	if err := os.WriteFile(cfg, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	for _, cmd := range []string{"status", "statusfull", "list", "pull", "fetch", "branch", "releases", "check"} {
		_, stderr, code := runGip(t, binPath, "-f", cfg, cmd, "frontend")
		if code != exitUsage || !strings.Contains(stderr, `unexpected argument "frontend"`) || !strings.Contains(stderr, "--tag") {
			t.Errorf("gip %s frontend: exit %d\n%s", cmd, code, stderr)
		}
	}

	_, stderr, _ := runGip(t, binPath, "-f", cfg, "status")
	if !strings.Contains(stderr, "gone: "+filepath.Join(root, "gone")+" does not exist (clone it with 'gip pull --all'") {
		t.Errorf("missing directory warning:\n%s", stderr)
	}
	if !strings.Contains(stderr, "notgit: "+notGit+" is not a git repository") {
		t.Errorf("not a repository warning:\n%s", stderr)
	}
	_, stderr, _ = runGip(t, binPath, "-f", cfg, "-m", "status")
	if strings.Contains(stderr, "does not exist") {
		t.Errorf("-m must hide the warning:\n%s", stderr)
	}

	t.Setenv("PATH", root) // no git here
	_, stderr, code := runGip(t, binPath, "-f", cfg, "status")
	if code != exitFailure || !strings.Contains(stderr, "git executable not found in PATH: install git") {
		t.Errorf("git missing: exit %d\n%s", code, stderr)
	}
}
