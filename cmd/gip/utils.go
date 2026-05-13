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
	Name       string `json:"name" yaml:"name"`
	Repository string `json:"repository" yaml:"repository"`
	LocalPath  string `json:"local_path" yaml:"local_path"`
	PullPolicy string `json:"pull_policy" yaml:"pull_policy"`
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
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Error retrieving user home: %v", err)
	}
	configurationFile := filepath.Join(home, configFileBaseName)
	ui.Confidentialf("Using configuration file %s", configurationFile)
	if !files.Exists(configurationFile) {
		return "", fmt.Errorf("Configuration file %s not found", configurationFile)
	}
	return configurationFile, nil
}

func normalizePath(dirpath string) string {
	return strings.TrimSuffix(filepath.ToSlash(dirpath), "/")
}

func projectsList(configurationPath string) ([]gipProject, error) {
	var projects []gipProject
	data, err := os.ReadFile(configurationPath)
	if err != nil {
		ui.Errorf("Error reading %s: %v", configurationPath, err)
		return projects, err
	}
	err = yaml.Unmarshal(data, &projects)
	if err != nil {
		ui.Errorf("Error reading configuration: %v", err)
		ui.Lifecyclef("Check the format of %s: it should be Yaml or Json", configurationPath)
		return projects, err
	}
	for _, p := range projects {
		if !p.isValidPullPolicy() {
			ui.Warnf("Project %q has unknown pull_policy %q (valid values: never, always)", p.Name, p.PullPolicy)
		}
		if p.Repository == "" {
			ui.Warnf(`No repository URL for project %s`, p.Name)
		} else if p.repoProvider() == "" {
			ui.Warnf(`Unable to detect provider for project %s using repository URL %s`, p.Name, p.Repository)
		}
	}
	return projects, nil
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

func isProjectDir(dirpath string) bool {
	return files.Exists(filepath.Join(dirpath, ".git"))
}
