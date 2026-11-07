package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPortableLocalPath(t *testing.T) {
	home := filepath.FromSlash("/home/u")
	work := filepath.Join(home, "work")
	cases := []struct {
		repo, configDir, want string
	}{
		{filepath.Join(work, "api"), work, "api"},
		{filepath.Join(work, "team", "api"), work, "team/api"},
		{filepath.Join(home, "other", "x"), work, "~/other/x"},
		{filepath.Join(home, "work", "api"), home, "~/work/api"},
		{filepath.Join(home, "x"), "", "~/x"},
		{filepath.FromSlash("/srv/repo"), work, filepath.FromSlash("/srv/repo")},
		{filepath.Join(home+"2", "x"), "", filepath.Join(home+"2", "x")},
	}
	for _, tc := range cases {
		if got := portableLocalPath(tc.repo, tc.configDir, home); got != tc.want {
			t.Errorf("portableLocalPath(%q, %q) = %q, want %q", tc.repo, tc.configDir, got, tc.want)
		}
	}
}

// makeOriginRepo creates a git repository with an origin remote.
func makeOriginRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "remote", "add", "origin", "https://example.com/"+filepath.Base(dir)+".git")
}

func readInitEntries(t *testing.T, data []byte) map[string]string {
	t.Helper()
	var entries []initEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, data)
	}
	m := map[string]string{}
	for _, e := range entries {
		m[e.Name] = e.LocalPath
	}
	return m
}

func TestInitPortableAndScriptable(t *testing.T) {
	root := t.TempDir()
	binPath := buildBinary(t, root)
	home := filepath.Join(root, "home")
	work := filepath.Join(home, "work")
	makeOriginRepo(t, filepath.Join(work, "api"))
	makeOriginRepo(t, filepath.Join(home, "dotfiles"))
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIP_FILE", "")

	// Relative to the output file below it, ~/ elsewhere under home.
	cfg := filepath.Join(work, ".gip")
	if _, stderr, code := runGip(t, binPath, "init", home, "-o", cfg); code != exitOK {
		t.Fatalf("init: exit %d\n%s", code, stderr)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := readInitEntries(t, data)
	if got["api"] != "api" || got["dotfiles"] != "~/dotfiles" {
		t.Errorf("local_path values: %v", got)
	}
	// The generated file works from any directory.
	if stdout, stderr, code := runGip(t, binPath, "-f", cfg, "--strict", "list"); code != exitOK {
		t.Fatalf("list of the generated file: exit %d\n%s%s", code, stdout, stderr)
	}

	// An existing file without a terminal: exit 2 with the ways out.
	_, stderr, code := runGip(t, binPath, "init", home, "-o", cfg)
	if code != exitUsage || !strings.Contains(stderr, "--force") || !strings.Contains(stderr, "-o -") {
		t.Errorf("existing file: exit %d\n%s", code, stderr)
	}

	// -o -: clean YAML on stdout, progress on stderr.
	stdout, stderr, code := runGip(t, binPath, "init", home, "-o", "-")
	if code != exitOK || !strings.Contains(stderr, "found api") || strings.Contains(stdout, "found") {
		t.Fatalf("-o -: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if got := readInitEntries(t, []byte(stdout)); got["api"] != "~/work/api" {
		t.Errorf("-o - local_path values: %v", got)
	}

	// --absolute keeps absolute paths.
	stdout, _, _ = runGip(t, binPath, "init", home, "-o", "-", "--absolute")
	if got := readInitEntries(t, []byte(stdout)); got["api"] != filepath.Join(work, "api") {
		t.Errorf("--absolute local_path values: %v", got)
	}

	// Not a directory: usage error.
	if _, _, code := runGip(t, binPath, "init", filepath.Join(root, "nope")); code != exitUsage {
		t.Errorf("init of a missing directory: exit %d", code)
	}
}
