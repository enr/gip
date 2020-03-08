package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/enr/go-commons/environment"
	"github.com/enr/go-files/files"
	"github.com/enr/runcmd"

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
		return ""
	}
	u, err := url.Parse(p.Repository)
	if err != nil {
		return ""
	}
	return u.Host
}

func gitExecutablePath() string {
	gitExecutable := environment.Which("git")
	if gitExecutable == "" {
		ui.Errorf("git not found in path. exit\n")
		os.Exit(1)
	}
	return gitExecutable
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

func executeGitStatus(dirpath string, untracked bool) {
	ui.Confidentialf("Status on %s", dirpath)
	command := &runcmd.Command{
		Exe:  gitExecutablePath(),
		Args: statusArguments(dirpath, untracked),
	}
	ui.Confidentialf("Execute command %s", command)
	result := command.Run()
	if !result.Success() {
		ui.Errorf("Error executing Git in %s", dirpath)
		ui.Errorf("(%d) %v", result.ExitStatus(), result.Error())
	}
	gitOutput := result.Stdout().String()
	if len(gitOutput) == 0 {
		ui.Confidentialf("%s unmodified", dirpath)
	} else {
		ui.Title(dirpath)
		fmt.Println(string(gitOutput))
	}
}

func statusArguments(dirpath string, untracked bool) []string {
	untrackedFlag := "=no"
	if untracked {
		untrackedFlag = ""
	}
	args := []string{
		fmt.Sprintf("--git-dir=%s/.git", dirpath),
		fmt.Sprintf("--work-tree=%s", dirpath),
		"status",
		"--porcelain",
		fmt.Sprintf("--untracked-files%s", untrackedFlag),
	}
	return args
}

func executeGitClone(repourl string, dirpath string) {
	ui.Confidentialf("Cloning %s to %s", repourl, dirpath)
	err := os.MkdirAll(dirpath, 0755)
	if err != nil {
		ui.Errorf("Error preparing for clone path %s:", dirpath)
		ui.Errorf("%v", err)
		return
	}
	args := []string{
		"clone",
		repourl,
		dirpath,
	}
	command := &runcmd.Command{
		Exe:  gitExecutablePath(),
		Args: args,
	}
	ui.Confidentialf("Execute command %s", command)
	result := command.Run()
	if !result.Success() {
		ui.Errorf("Error executing Git in %s", dirpath)
		ui.Errorf("(%d) %v", result.ExitStatus(), result.Error())
	}
	gitOutput := result.Stdout().String()
	ui.Title(dirpath)
	fmt.Println(string(gitOutput))
}

func executeGitPull(dirpath string) {
	ui.Confidentialf("Pulling %s", dirpath)
	args := []string{
		fmt.Sprintf("--git-dir=%s/.git", dirpath),
		"pull",
	}
	command := &runcmd.Command{
		Exe:        gitExecutablePath(),
		Args:       args,
		WorkingDir: dirpath,
	}
	ui.Confidentialf("Execute command %s", command)
	result := command.Run()
	if !result.Success() {
		ui.Errorf("Error executing Git in %s", dirpath)
		ui.Errorf("(%d) %v", result.ExitStatus(), result.Error())
	}
	gitOutput := result.Stdout().String()
	ui.Title(dirpath)
	fmt.Println(string(gitOutput))
}
