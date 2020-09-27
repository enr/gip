package main

import (
	"io/ioutil"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/enr/go-files/files"

	"github.com/mitchellh/go-homedir"
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
	return "never" == strings.ToLower(strings.TrimSpace(p.PullPolicy))
}

func (p *gipProject) pullAlways() bool {
	return "always" == strings.ToLower(strings.TrimSpace(p.PullPolicy))
}

func (p *gipProject) repoProvider() string {
	if p.Repository == "" {
		ui.Warnf(`No repository URL for project %s`, p.Name)
		return ""
	}
	u, err := url.Parse(p.Repository)
	if err != nil {
		ui.Warnf(`Error parsing %s repository URL %s : %v`, p.Name, p.Repository, err)
		return ""
	}
	if u.Host == "" {
		ui.Warnf(`Unable to detect provider for project %s using repository URL %s`, p.Name, p.Repository)
	}
	return u.Host
}

func defaultConfigurationFilePath() string {
	home, err := homedir.Dir()
	if err != nil {
		ui.Errorf("Error retrieving user home: %v\n", err)
		os.Exit(1)
	}
	configurationFile := filepath.Join(home, configFileBaseName)
	ui.Confidentialf("Using configuration file %s", configurationFile)
	if !files.Exists(configurationFile) {
		ui.Errorf("Configuration file %s not found. Exit", configurationFile)
		os.Exit(1)
	}
	return configurationFile
}

func normalizePath(dirpath string) string {
	return strings.TrimSuffix(filepath.ToSlash(dirpath), "/")
}

func projectsList(configurationPath string) ([]gipProject, error) {
	var projects []gipProject
	bytes, err := ioutil.ReadFile(configurationPath)
	if err != nil {
		ui.Errorf("Error reading %s: %v", configurationPath, err)
		return projects, err
	}
	err = yaml.Unmarshal(bytes, &projects)
	if err != nil {
		ui.Errorf("Error reading configuration: %v", err)
		ui.Lifecyclef("Check the format of %s: it should be Yaml or Json", configurationPath)
		return projects, err
	}
	return projects, nil
}

func projectPath(ppath string) (string, error) {
	retpath := ppath
	if strings.HasPrefix(ppath, "~") {
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
	return files.IsDir(filepath.Join(dirpath, ".git"))
}
