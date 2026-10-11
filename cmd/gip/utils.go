package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/enr/go-files/files"
)

const (
	configFileBaseName = ".gip"
)

type gipProject struct {
	Name       string   `json:"name" yaml:"name"`
	Repository string   `json:"repository" yaml:"repository"`
	LocalPath  string   `json:"local_path" yaml:"local_path"`
	PullPolicy string   `json:"pull_policy" yaml:"pull_policy"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Branches lists extra local branches kept aligned with their upstream by
	// status (report) and pull (fast-forward only). Empty: only the checked-out
	// branch is considered, as before.
	Branches []string `json:"branches,omitempty" yaml:"branches,omitempty"`
	Disabled bool     `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	// baseDir is the directory of the configuration file the project was read
	// from: relative local_path values are resolved against it.
	baseDir string
}

// resolvedPath returns the project directory: "~" and environment variables
// are expanded and a relative local_path is taken from the configuration
// file's directory, so the result does not depend on where gip is run from.
func (p *gipProject) resolvedPath() (string, error) {
	resolved, err := projectPath(p.LocalPath)
	if err != nil {
		return "", err
	}
	resolved = filepath.FromSlash(resolved)
	if p.baseDir != "" && !isRootedPath(resolved) {
		resolved = filepath.Join(p.baseDir, resolved)
	}
	return resolved, nil
}

// isRootedPath reports whether path must not be joined to a base directory.
// On Windows filepath.IsAbs is false for "\tmp\x" (no drive) and "C:x"
// (drive-relative), but neither is relative to the configuration file.
func isRootedPath(path string) bool {
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return true
	}
	return path != "" && os.IsPathSeparator(path[0])
}

func (p *gipProject) pullNever() bool {
	return strings.EqualFold(strings.TrimSpace(p.PullPolicy), "never")
}

func (p *gipProject) pullAlways() bool {
	return strings.EqualFold(strings.TrimSpace(p.PullPolicy), "always")
}

func (p *gipProject) isValidPullPolicy() bool {
	v := strings.TrimSpace(p.PullPolicy)
	return v == "" || strings.EqualFold(v, "never") || strings.EqualFold(v, "always")
}

// extraBranches returns the configured branches, trimmed and de-duplicated,
// excluding the current one.
func (p *gipProject) extraBranches(current string) []string {
	var out []string
	seen := map[string]bool{current: true}
	for _, b := range p.Branches {
		b = strings.TrimSpace(b)
		if b == "" || strings.HasPrefix(b, "-") || seen[b] {
			continue
		}
		seen[b] = true
		out = append(out, b)
	}
	return out
}

func (p *gipProject) repoProvider() string {
	if p.Repository == "" {
		return ""
	}
	// SCP-style SSH URLs: [user@]host:path — not handled by url.Parse
	if !strings.Contains(p.Repository, "://") {
		if at := strings.Index(p.Repository, "@"); at >= 0 {
			hostPath := p.Repository[at+1:]
			if colon := strings.Index(hostPath, ":"); colon >= 0 {
				return hostPath[:colon]
			}
		}
		return ""
	}
	u, err := url.Parse(p.Repository)
	if err != nil {
		return ""
	}
	return u.Host
}

func defaultConfigurationFilePath() (string, error) {
	// 1. GIP_FILE environment variable
	if envPath := os.Getenv("GIP_FILE"); envPath != "" {
		abs, err := filepath.Abs(envPath)
		if err != nil {
			return "", fmt.Errorf("invalid GIP_FILE value: %v", err)
		}
		if !files.Exists(abs) {
			return "", fmt.Errorf("GIP_FILE path %s not found", abs)
		}
		ui.Confidentialf("Using configuration file from GIP_FILE: %s", abs)
		return abs, nil
	}

	// 2. .gip in current working directory
	if cwd, err := os.Getwd(); err == nil {
		local := filepath.Join(cwd, configFileBaseName)
		if files.Exists(local) {
			ui.Confidentialf("Using local configuration file: %s", local)
			return local, nil
		}
	}

	// 3. ~/.gip
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error retrieving user home: %v", err)
	}
	configFile := filepath.Join(home, configFileBaseName)
	ui.Confidentialf("Using configuration file %s", configFile)
	if !files.Exists(configFile) {
		return "", fmt.Errorf("configuration file not found (searched: ./.gip, %s)", configFile)
	}
	return configFile, nil
}

func normalizePath(dirpath string) string {
	return strings.TrimSuffix(filepath.ToSlash(dirpath), "/")
}

func projectsList(configurationPath string) ([]gipProject, []string, error) {
	var projects []gipProject
	var warnings []string

	warn := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		ui.Warnf("%s", msg)
		warnings = append(warnings, msg)
	}

	data, err := os.ReadFile(configurationPath)
	if err != nil {
		ui.Errorf("Error reading %s: %v", configurationPath, err)
		return projects, warnings, err
	}
	// Unknown keys are rejected: a typo such as "pull-policy" would otherwise be
	// dropped silently and the project would run with the default behaviour.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err = dec.Decode(&projects); err != nil && !errors.Is(err, io.EOF) {
		ui.Errorf("Error reading configuration: %v", err)
		ui.Lifecyclef("Check the format of %s: it should be Yaml or Json", configurationPath)
		return projects, warnings, err
	}

	baseDir := filepath.Dir(configurationPath)
	if abs, absErr := filepath.Abs(baseDir); absErr == nil {
		baseDir = abs
	}
	for i := range projects {
		projects[i].baseDir = baseDir
	}

	nameIndex := make(map[string]int) // tracks first occurrence index of each name
	for i, p := range projects {
		if strings.TrimSpace(p.Name) == "" {
			warn("project #%d has empty name", i+1)
		}
		if strings.TrimSpace(p.LocalPath) == "" {
			warn("project %q (#%d) has empty local_path", p.Name, i+1)
		}
		if !p.isValidPullPolicy() {
			warn("project %q has unknown pull_policy %q (valid values: never, always)", p.Name, p.PullPolicy)
		}
		for _, b := range p.Branches {
			if v := strings.TrimSpace(b); v == "" || strings.HasPrefix(v, "-") {
				warn("project %q has invalid branch %q in branches (ignored)", p.Name, b)
			}
		}
		if p.Repository == "" {
			warn("no repository URL for project %s", p.Name)
		} else if p.repoProvider() == "" {
			warn("unable to detect provider for project %s using repository URL %s", p.Name, p.Repository)
		}
		if name := strings.TrimSpace(p.Name); name != "" {
			if prevIdx, already := nameIndex[name]; already {
				return nil, warnings, fmt.Errorf("%s: duplicate project name %q (entries #%d and #%d)", configurationPath, name, prevIdx+1, i+1)
			}
			nameIndex[name] = i
		}
	}
	return projects, warnings, nil
}

func projectPath(ppath string) (string, error) {
	// An empty path would resolve to the current working directory and make
	// gip operate on whatever repository it happens to be launched from.
	if strings.TrimSpace(ppath) == "" {
		return "", fmt.Errorf("empty local_path")
	}
	retpath := ppath
	if ppath == "~" || strings.HasPrefix(ppath, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		dir := usr.HomeDir
		relpath := strings.TrimPrefix(ppath, "~")
		retpath = filepath.FromSlash(path.Join(dir, relpath))
	}
	// An unset variable must not expand to "": "$UNSET/x" would become "/x".
	var missing []string
	expanded := os.Expand(retpath, func(name string) string {
		v, ok := os.LookupEnv(name)
		if !ok || v == "" {
			missing = append(missing, name)
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("environment variable %s not set or empty in path %q", strings.Join(missing, ", "), ppath)
	}
	return expanded, nil
}

// filterDisabled removes projects that have Disabled set to true.
func filterDisabled(projects []gipProject) []gipProject {
	out := projects[:0:0]
	for _, p := range projects {
		if !p.Disabled {
			out = append(out, p)
		}
	}
	return out
}

// filterByTag returns the subset of projects that carry at least one of the
// comma-separated tags in tagList. An empty tagList returns all projects.
func filterByTag(projects []gipProject, tagList string) []gipProject {
	tagList = strings.TrimSpace(tagList)
	if tagList == "" {
		return projects
	}
	wanted := make(map[string]bool)
	for _, t := range strings.Split(tagList, ",") {
		if v := strings.TrimSpace(t); v != "" {
			wanted[v] = true
		}
	}
	var out []gipProject
	for _, p := range projects {
		for _, t := range p.Tags {
			if wanted[strings.TrimSpace(t)] {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

func isProjectDir(dirpath string) bool {
	return files.Exists(filepath.Join(dirpath, ".git"))
}
