package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkipKindOf(t *testing.T) {
	cases := map[string]skipKind{
		reasonNotProjectDir: skipMissing,
		reasonDirMissing:    skipMissing,
		reasonBranchMissing: skipMissing,
		reasonPullNever:     skipProtected,
		reasonUncommitted:   skipProtected,
		"not behind":        skipFiltered,
		"not dirty":         skipFiltered,
		"in sync":           skipFiltered,
	}
	for reason, want := range cases {
		if got := skipKindOf(reason); got != want {
			t.Errorf("skipKindOf(%q) = %s, want %s", reason, got, want)
		}
	}
}

func TestStrictResult(t *testing.T) {
	missing := opResult{project: "p", status: opSkipped, reason: reasonDirMissing}
	filtered := opResult{project: "p", status: opSkipped, reason: "not dirty"}

	strictMode = false
	if r := strictResult(missing); r.status != opSkipped {
		t.Errorf("without --strict a missing directory is skipped, got %v", r.status)
	}
	strictMode = true
	defer func() { strictMode = false }()
	if r := strictResult(missing); r.status != opError || !strings.Contains(r.err.Error(), "--strict") {
		t.Errorf("with --strict a missing directory is an error, got %+v", r)
	}
	if r := strictResult(filtered); r.status != opSkipped {
		t.Errorf("--strict must not change filtered projects, got %v", r.status)
	}
}

func TestCountsSkipDetail(t *testing.T) {
	tr := newTracker(4)
	tr.results = []opResult{
		{status: opOK},
		{status: opSkipped, reason: reasonDirMissing},
		{status: opSkipped, reason: "not dirty"},
		{status: opSkipped, reason: "not ahead"},
	}
	n := tr.counts()
	if n.ok != 1 || n.skipped != 3 || n.skippedBy[skipMissing] != 1 || n.skippedBy[skipProtected] != 0 || n.skippedBy[skipFiltered] != 2 {
		t.Fatalf("counts = %+v", n)
	}
	if got := n.skipDetail(); got != " (missing 1, filtered 2)" {
		t.Errorf("skipDetail = %q", got)
	}
	if got := (resultCounts{}).skipDetail(); got != "" {
		t.Errorf("nothing skipped: skipDetail = %q", got)
	}
}

// TestExitCodes checks the three exit statuses: 0 success, 1 failed projects
// (or missing ones with --strict), 2 usage and configuration errors.
func TestExitCodes(t *testing.T) {
	root := t.TempDir()
	binPath := buildBinary(t, root)
	t.Setenv("GIP_FILE", "")
	t.Setenv("HOME", root)

	repo := filepath.Join(root, "repo")
	runGit(t, root, "init", "-q", repo)
	broken := filepath.Join(root, "broken")
	if err := os.MkdirAll(filepath.Join(broken, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) string {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	okCfg := write("ok.yml", "- name: repo\n  local_path: "+repo+"\n  repository: x\n")
	missingCfg := write("missing.yml", "- name: repo\n  local_path: "+repo+"\n  repository: x\n- name: gone\n  local_path: "+filepath.Join(root, "gone")+"\n  repository: x\n")
	goneCfg := write("gone.yml", "- name: gone\n  local_path: "+filepath.Join(root, "gone")+"\n  repository: x\n")
	brokenCfg := write("broken.yml", "- name: broken\n  local_path: "+broken+"\n  repository: x\n")

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"success", []string{"-f", okCfg, "status"}, exitOK},
		{"missing directory is skipped", []string{"-f", missingCfg, "status"}, exitOK},
		{"missing directory with --strict", []string{"-f", missingCfg, "--strict", "status"}, exitFailure},
		{"--strict after the command", []string{"-f", missingCfg, "branch", "--strict"}, exitFailure},
		{"list with --strict", []string{"-f", missingCfg, "--strict", "list"}, exitFailure},
		{"check warns about a missing directory", []string{"-f", goneCfg, "check"}, exitOK},
		{"check with --strict", []string{"-f", goneCfg, "check", "--strict"}, exitFailure},
		{"failed project", []string{"-f", brokenCfg, "status"}, exitFailure},
		{"configuration not found", []string{"status"}, exitUsage},
		{"configuration file not readable", []string{"-f", filepath.Join(root, "nope.yml"), "status"}, exitUsage},
		{"bad flag value", []string{"-f", okCfg, "status", "--jobs", "0"}, exitUsage},
		{"unknown flag", []string{"-f", okCfg, "status", "--nope"}, exitUsage},
		{"unknown command", []string{"-f", okCfg, "stauts"}, exitUsage},
		{"missing exec command", []string{"-f", okCfg, "exec"}, exitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runGip(t, binPath, tc.args...)
			if code != tc.want {
				t.Fatalf("gip %v: exit %d, want %d\nstdout: %s\nstderr: %s", tc.args, code, tc.want, stdout, stderr)
			}
		})
	}

	// The summary splits the skipped projects; --strict reports the missing one as an error.
	stdout, _, _ := runGip(t, binPath, "-f", missingCfg, "status")
	if !strings.Contains(stdout, "Skipped: 1 (missing 1)") {
		t.Errorf("summary must split the skipped count:\n%s", stdout)
	}
	stdout, _, _ = runGip(t, binPath, "-f", missingCfg, "--json", "--strict", "status")
	var env struct {
		Projects []map[string]interface{} `json:"projects"`
		Summary  struct {
			Errors    int            `json:"errors"`
			Skipped   int            `json:"skipped"`
			SkippedBy map[string]int `json:"skipped_by"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if env.Summary.Errors != 1 || env.Summary.Skipped != 0 || len(env.Summary.SkippedBy) != 3 {
		t.Errorf("--strict summary: %+v", env.Summary)
	}

	stdout, _, _ = runGip(t, binPath, "-f", missingCfg, "--json", "status")
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if env.Summary.SkippedBy["missing"] != 1 || env.Projects[0]["skip"] != "missing" {
		t.Errorf("skip kind: summary %+v, projects %v", env.Summary, env.Projects)
	}
}

func TestMissingReason(t *testing.T) {
	root := t.TempDir()
	if got := missingReason(filepath.Join(root, "nope")); got != reasonDirMissing {
		t.Errorf("missing directory: %q", got)
	}
	if got := missingReason(root); got != reasonNotProjectDir {
		t.Errorf("directory without .git: %q", got)
	}
}

func TestSkipNotes(t *testing.T) {
	skipped := []opResult{
		{project: "z", status: opSkipped, reason: reasonDirMissing},
		{project: "b", status: opSkipped, reason: "not dirty"},
		{project: "a", status: opSkipped, reason: reasonNotProjectDir},
		{project: "p", status: opSkipped, reason: reasonUncommitted},
	}
	want := []string{
		"  missing: a (not a git repository), z (directory does not exist)",
		"  protected: p (uncommitted changes, use --force)",
	}
	if got := skipNotes(skipped); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("skipNotes:\n got: %q\nwant: %q", got, want)
	}
	if got := skipNotes([]opResult{{project: "b", reason: "not dirty"}}); len(got) != 0 {
		t.Errorf("filtered projects need no note, got %q", got)
	}

	var many []opResult
	for i := 0; i < maxSkipNotes+2; i++ {
		many = append(many, opResult{project: string(rune('a' + i)), reason: reasonDirMissing})
	}
	got := skipNotes(many)
	if len(got) != 1 || !strings.HasSuffix(got[0], ", +2 more (--show-skipped)") {
		t.Errorf("long list must be capped: %q", got)
	}
}
