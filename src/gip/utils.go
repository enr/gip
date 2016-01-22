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
	ConfigFileBaseName = ".gip"
)

type Project struct {
	Name       string
	Repository string
	LocalPath  string
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
	configurationFile := filepath.Join(home, ConfigFileBaseName)
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

func projectsList(configurationPath string) ([]Project, error) {
	var appz []Project
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
	return retpath, nil
}

func isProjectDir(dirpath string) bool {
	return files.IsDir(filepath.Join(dirpath, ".git"))
}

func executeGitStatus(dirpath string, untracked bool) {
	command := &runcmd.Command{
		Exe:  gitExecutablePath(),
		Args: arguments(dirpath, untracked),
	}
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

func arguments(dirpath string, untracked bool) []string {
	// git --git-dir=${prj_dir}/.git --work-tree=${prj_dir} ${git_action}
	// status --porcelain --untracked-files=no
	untrackedFlag := "=no"
	if untracked {
		untrackedFlag = ""
	}
	args := []string{
		fmt.Sprintf("--git-dir=%s/.git", dirpath),
		//fmt.Sprintf("--git-dir=%s", filepath.Join(dirpath, ".git")),
		fmt.Sprintf("--work-tree=%s", dirpath),
		"status",
		"--porcelain",
		fmt.Sprintf("--untracked-files%s", untrackedFlag),
	}
	return args
}
