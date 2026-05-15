package main

import (
	"fmt"
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
	Disabled   bool     `json:"disabled,omitempty" yaml:"disabled,omitempty"`
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
	err = yaml.Unmarshal(data, &projects)
	if err != nil {
		ui.Errorf("Error reading configuration: %v", err)
		ui.Lifecyclef("Check the format of %s: it should be Yaml or Json", configurationPath)
		return projects, warnings, err
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
	return os.ExpandEnv(retpath), nil
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
