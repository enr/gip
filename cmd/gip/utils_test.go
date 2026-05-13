package main

import (
	"flag"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/enr/clui"
	"github.com/urfave/cli/v2"
)

func TestProjectPath(t *testing.T) {
	usr, err := user.Current()
	if err != nil {
		t.Fatalf("cannot get current user: %v", err)
	}
	home := usr.HomeDir

	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		// bare ~ expands to home
		{"~", home, false},
		// ~/foo expands to home/foo
		{"~/foo", filepath.Join(home, "foo"), false},
		// ~bob/foo must NOT expand using current user's home
		{"~bob/foo", "~bob/foo", false},
		// ~stuff (no slash) must NOT expand
		{"~stuff", "~stuff", false},
		// absolute path passes through
		{"/tmp/bar", "/tmp/bar", false},
	}
	for _, tc := range cases {
		got, err := projectPath(tc.input)
		if tc.wantErr && err == nil {
			t.Errorf("projectPath(%q): expected error, got nil", tc.input)
			continue
		}
		if !tc.wantErr && err != nil {
			t.Errorf("projectPath(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("projectPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRepoProviderSSHURL(t *testing.T) {
	if ui == nil {
		ui, _ = clui.NewClui(func(u *clui.Clui) {
			u.VerbosityLevel = clui.VerbosityLevelLow
		})
	}
	cases := []struct {
		repo     string
		expected string
	}{
		{"git@github.com:enr/gip.git", "github.com"},
		{"git@gitlab.com:user/repo.git", "gitlab.com"},
		{"https://github.com/enr/gip.git", "github.com"},
	}
	for _, tc := range cases {
		p := &gipProject{Name: "test", Repository: tc.repo}
		got := p.repoProvider()
		if got != tc.expected {
			t.Errorf("repoProvider(%q) = %q, want %q", tc.repo, got, tc.expected)
		}
	}
}

func TestIsProjectDir(t *testing.T) {
	// normal repo: .git is a directory
	dirRepo := t.TempDir()
	if err := os.Mkdir(filepath.Join(dirRepo, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if !isProjectDir(dirRepo) {
		t.Errorf("isProjectDir(%q): expected true for dir-based .git, got false", dirRepo)
	}

	// worktree/submodule: .git is a file
	fileRepo := t.TempDir()
	f, err := os.Create(filepath.Join(fileRepo, ".git"))
	if err != nil {
		t.Fatalf("create .git file: %v", err)
	}
	f.Close()
	if !isProjectDir(fileRepo) {
		t.Errorf("isProjectDir(%q): expected true for file-based .git (worktree/submodule), got false", fileRepo)
	}

	// not a repo: no .git at all
	noRepo := t.TempDir()
	if isProjectDir(noRepo) {
		t.Errorf("isProjectDir(%q): expected false for dir without .git, got true", noRepo)
	}
}

func TestConfigParsingYaml(t *testing.T) {
	fp := "../../testdata/linux.yaml"
	projects, err := projectsList(fp)
	if err != nil {
		t.Errorf("Unexpected error reading %s: %v", fp, err)
	}
	if len(projects) != 3 {
		t.Errorf("In %s expected 3 projects, got %d", fp, len(projects))
	}
}

func TestConfigParsingJson(t *testing.T) {
	fp := "../../testdata/linux.json"
	projects, err := projectsList(fp)
	if err != nil {
		t.Errorf("Unexpected error reading %s: %v", fp, err)
	}
	if len(projects) != 3 {
		t.Errorf("In %s expected 3 projects, got %d", fp, len(projects))
	}
}

func TestConfigurationFilePathMissing(t *testing.T) {
	// Temporarily override HOME to an empty temp dir
	// so that .gip is not found.
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome) // For Windows if tested there

	// Initialize ui to avoid panics during test
	if ui == nil {
		ui, _ = clui.NewClui(func(ui *clui.Clui) {
			ui.VerbosityLevel = clui.VerbosityLevelLow
		})
	}
	app := &cli.App{}
	app.Before = func(c *cli.Context) error { return nil } // just so it's not nil if needed

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := cli.NewContext(app, fs, nil)

	_, err := configurationFilePath(c)
	if err == nil {
		t.Errorf("Expected an error when configuration file is missing, got nil")
	}
}
