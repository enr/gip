package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/enr/go-commons/environment"
	"github.com/enr/go-files/files"
	"github.com/enr/runcmd"
)

const (
	configFileBaseName = ".gip"
)

type gipProject struct {
	Name       string
	Repository string
	LocalPath  string
	PullPolicy string
}

func (p *gipProject) pullNever() bool {
	return "never" == strings.ToLower(strings.TrimSpace(p.PullPolicy))
}

func (p *gipProject) pullAlways() bool {
	return "always" == strings.ToLower(strings.TrimSpace(p.PullPolicy))
}

func gitExecutablePath() string {
	gitExecutable := environment.Which("git")
	if gitExecutable == "" {
		ui.Errorf("git not found in path. exit\n")
		os.Exit(1)
	}
	return gitExecutable
}

func configurationFilePath() string {
	home, err := environment.UserHome()
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
	var appz []gipProject
	bytes, err := ioutil.ReadFile(configurationPath)
	if err != nil {
		ui.Errorf("Error reading %s: %v", configurationPath, err)
		return appz, err
	}
	err = json.Unmarshal(bytes, &appz)
	if err != nil {
		ui.Errorf("Error reading configuration: %v", err)
		ui.Lifecyclef("Check the format of %s: it should be Json", configurationPath)
		return appz, err
	}
	return appz, nil
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
		ui.Errorf("Error executing Git (%d): %v", result.ExitStatus(), result.Error())
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
		ui.Errorf("Error executing Git (%d): %v", result.ExitStatus(), result.Error())
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
		Exe:  gitExecutablePath(),
		Args: args,
	}
	ui.Confidentialf("Execute command %s", command)
	result := command.Run()
	if !result.Success() {
		ui.Errorf("Error executing Git (%d): %v", result.ExitStatus(), result.Error())
	}
	gitOutput := result.Stdout().String()
	ui.Title(dirpath)
	fmt.Println(string(gitOutput))
}
