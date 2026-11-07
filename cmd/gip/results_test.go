package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/enr/gip/lib/core"
)

func TestParsePorcelain(t *testing.T) {
	out := " M changed.go\nA  added.go\n?? new file.txt\nR  old.go -> renamed.go\n?? \"caf\\303\\251.txt\"\nMM both.go\n"
	want := []statusFile{
		{Path: "changed.go", Index: ".", Worktree: "M"},
		{Path: "added.go", Index: "A", Worktree: "."},
		{Path: "new file.txt", Index: "?", Worktree: "?"},
		{Path: "renamed.go", OrigPath: "old.go", Index: "R", Worktree: "."},
		{Path: "café.txt", Index: "?", Worktree: "?"},
		{Path: "both.go", Index: "M", Worktree: "M"},
	}
	if got := parsePorcelain(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePorcelain:\n got: %+v\nwant: %+v", got, want)
	}
	if got := parsePorcelain(""); got == nil || len(got) != 0 {
		t.Fatalf("a clean tree must give an empty, non-nil slice, got %#v", got)
	}
}

func TestOpResultToJSON(t *testing.T) {
	st := repoState{
		sync:  core.BranchSyncStatus{Ahead: 0, Behind: 2},
		dirty: core.DirtyStatus{Untracked: true},
	}
	r := opResult{project: "p", status: opOK, state: &st, files: []statusFile{}, execRes: &core.ExecResult{ExitCode: 3, Stdout: "o"},
		pull: &pullInfo{from: "a", to: "b", commits: 2}}
	data, err := json.Marshal(r.toJSON())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	state := got["state"].(map[string]interface{})
	wantState := map[string]interface{}{"upstream": true, "ahead": 0.0, "behind": 2.0, "dirty": true,
		"staged": false, "unstaged": false, "untracked": true, "stashed": false}
	if !reflect.DeepEqual(state, wantState) {
		t.Errorf("state:\n got: %v\nwant: %v", state, wantState)
	}
	if files, ok := got["files"].([]interface{}); !ok || len(files) != 0 {
		t.Errorf("a clean tree must give \"files\": [], got %v", got["files"])
	}
	if ex := got["exec"].(map[string]interface{}); ex["exit_code"] != 3.0 || ex["stdout"] != "o" || ex["stderr"] != "" {
		t.Errorf("exec: %v", ex)
	}
	if pl := got["pull"].(map[string]interface{}); pl["updated"] != true || pl["commits"] != 2.0 || pl["cloned"] != false {
		t.Errorf("pull: %v", pl)
	}
	for _, k := range []string{"dirty", "ahead", "behind", "no_remote"} {
		if _, ok := got[k]; ok {
			t.Errorf("schema 2 has no top-level %q", k)
		}
	}

	data, _ = json.Marshal(opResult{project: "q", status: opSkipped}.toJSON())
	if string(data) != `{"name":"q","local_path":"","status":"skipped"}` {
		t.Errorf("unread data must be omitted, got %s", data)
	}
}

// jsonEnvelope runs gip --json and returns the whole decoded envelope.
func jsonEnvelope(t *testing.T, binPath, cfg string, args ...string) map[string]interface{} {
	t.Helper()
	out, err := exec.Command(binPath, append([]string{"-f", cfg, "--json"}, args...)...).Output()
	var ee *exec.ExitError
	if err != nil && !errors.As(err, &ee) {
		t.Fatalf("gip %v: %v", args, err)
	}
	var env map[string]interface{}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("invalid JSON from gip %v: %v\n%s", args, err, out)
	}
	return env
}

// checkEnvelope verifies the envelope metadata and the order of the projects.
func checkEnvelope(t *testing.T, env map[string]interface{}, command string) {
	t.Helper()
	if env["schema_version"] != float64(jsonSchemaVersion) || env["command"] != command {
		t.Fatalf("envelope: schema_version=%v command=%v", env["schema_version"], env["command"])
	}
	var names []string
	for _, p := range env["projects"].([]interface{}) {
		names = append(names, p.(map[string]interface{})["name"].(string))
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("projects not sorted by name: %v", names)
	}
}

func envelopeProjects(env map[string]interface{}) map[string]map[string]interface{} {
	m := map[string]map[string]interface{}{}
	for _, p := range env["projects"].([]interface{}) {
		pm := p.(map[string]interface{})
		m[pm["name"].(string)] = pm
	}
	return m
}

func TestJSONCarriesResults(t *testing.T) {
	root := t.TempDir()
	binPath := buildBinary(t, root)
	cfg := setupFilterRepos(t, root)

	// statusfull: envelope metadata, stable order, files and state.
	env := jsonEnvelope(t, binPath, cfg, "statusfull")
	checkEnvelope(t, env, "statusfull")
	ps := envelopeProjects(env)
	untracked := ps["untracked"]
	files := untracked["files"].([]interface{})
	if len(files) != 1 {
		t.Fatalf("untracked: expected one file, got %v", untracked)
	}
	f := files[0].(map[string]interface{})
	if f["path"] != "new" || f["index"] != "?" || f["worktree"] != "?" {
		t.Errorf("untracked file: %v", f)
	}
	if st := untracked["state"].(map[string]interface{}); st["dirty"] != true || st["untracked"] != true {
		t.Errorf("untracked state: %v", st)
	}
	ahead := ps["ahead"]
	if fs, ok := ahead["files"].([]interface{}); !ok || len(fs) != 0 {
		t.Errorf("ahead (clean, unsynced): expected \"files\": [], got %v", ahead["files"])
	}
	if st := ahead["state"].(map[string]interface{}); st["ahead"] != 1.0 || st["dirty"] != false {
		t.Errorf("ahead state: %v", st)
	}

	// status (no untracked files): the untracked repo has nothing to list.
	ps = envelopeProjects(jsonEnvelope(t, binPath, cfg, "status"))
	if fs, ok := ps["untracked"]["files"].([]interface{}); !ok || len(fs) != 0 {
		t.Errorf("status: untracked files must not be listed, got %v", ps["untracked"])
	}
}

func TestExecJSONReportsOutput(t *testing.T) {
	root := t.TempDir()
	binPath := buildBinary(t, root)
	cfg := setupFilterRepos(t, root)

	// exec: stdout, stderr and exit code of every run.
	ps := envelopeProjects(jsonEnvelope(t, binPath, cfg, "exec", "--", "git", "rev-parse", "--abbrev-ref", "HEAD"))
	if ex := ps["ahead"]["exec"].(map[string]interface{}); ex["exit_code"] != 0.0 || ex["stdout"] != "main\n" {
		t.Errorf("exec ok: %v", ex)
	}
	ps = envelopeProjects(jsonEnvelope(t, binPath, cfg, "exec", "--", "git", "rev-parse", "--verify", "nope"))
	if ex := ps["ahead"]["exec"].(map[string]interface{}); ex["exit_code"] == 0.0 || ps["ahead"]["status"] != "error" {
		t.Errorf("exec failing: %v", ps["ahead"])
	}
}

func TestPullJSONReportsCommits(t *testing.T) {
	root := t.TempDir()
	binPath := buildBinary(t, root)
	setupFilterRepos(t, root)

	seed := filepath.Join(root, "seed")
	for i := 0; i < 2; i++ {
		runGit(t, seed, "commit", "-q", "--allow-empty", "-m", fmt.Sprintf("remote %d", i))
	}
	runGit(t, seed, "push", "-q", "origin", "main")

	// Only the repositories that can fast-forward ("ahead" has diverged).
	cfg := filepath.Join(root, "pull.yml")
	content := fmt.Sprintf("- name: untracked\n  local_path: %s\n  repository: x\n- name: missing\n  local_path: %s\n  repository: %s\n",
		filepath.Join(root, "untracked"), filepath.Join(root, "clone"), filepath.Join(root, "origin.git"))
	if err := os.WriteFile(cfg, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	ps := envelopeProjects(jsonEnvelope(t, binPath, cfg, "pull", "--force", "--all"))
	pl, ok := ps["untracked"]["pull"].(map[string]interface{})
	if !ok || pl["updated"] != true || pl["commits"] != 2.0 || pl["from"] == pl["to"] {
		t.Errorf("pull: %v", ps["untracked"])
	}
	cl, ok := ps["missing"]["pull"].(map[string]interface{})
	if !ok || cl["cloned"] != true || cl["to"] == "" {
		t.Errorf("clone: %v", ps["missing"])
	}

	// A second pull changes nothing.
	ps = envelopeProjects(jsonEnvelope(t, binPath, cfg, "pull", "--force"))
	if pl := ps["untracked"]["pull"].(map[string]interface{}); pl["updated"] != false || pl["commits"] != 0.0 {
		t.Errorf("pull up to date: %v", pl)
	}
}
