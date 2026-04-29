package main

import (
	"flag"
	"testing"

	"github.com/enr/clui"
	"github.com/urfave/cli/v2"
)

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
