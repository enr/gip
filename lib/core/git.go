package core

import (
	"fmt"
	"os"

	"github.com/enr/clui"
	"github.com/enr/go-commons/environment"
)

// NewGit is the factory function for GitCommands
func NewGit(ui *clui.Clui) (*GitCommands, error) {
	executor, err := newGitExecutor(ui)
	if err != nil {
		return &GitCommands{}, err
	}
	return &GitCommands{
		ui:       ui,
		executor: executor,
	}, nil
}

// GitCommands ...
type GitCommands struct {
	ui       *clui.Clui
	executor runcmdWrapper
}

// Clone executes `git clone`
func (g *GitCommands) Clone(repourl string, dirpath string) error {
	var err error
	g.ui.Confidentialf("Cloning %s to %s", repourl, dirpath)
	err = os.MkdirAll(dirpath, 0755)
	if err != nil {
		g.ui.Errorf("Error preparing for clone path %s:", dirpath)
		g.ui.Errorf("%v", err)
		return err
	}
	args := []string{
		"clone",
		repourl,
		dirpath,
	}
	// git, err := gitExecutablePath()
	// if err != nil {
	// 	return err
	// }
	// command := &runcmd.Command{
	// 	Exe:  git,
	// 	Args: args,
	// }
	// g.ui.Confidentialf("Execute command %s", command)
	r := runcmdWrapperRequest{
		args: args,
	}
	result := g.executor.exec(r)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("(%d) %v", result.ExitStatus(), result.Error())
		err = result.Error()
	}
	gitOutput := result.Stdout().String()
	g.ui.Title(dirpath)
	fmt.Println(string(gitOutput))
	return err
}

// Pull executes `git pull`
func (g *GitCommands) Pull(dirpath string) error {
	var err error
	g.ui.Confidentialf("Pulling %s", dirpath)
	args := []string{
		fmt.Sprintf("--git-dir=%s/.git", dirpath),
		"pull",
	}
	// git, err := gitExecutablePath()
	// if err != nil {
	// 	return err
	// }
	// command := &runcmd.Command{
	// 	Exe:        git,
	// 	Args:       args,
	// 	WorkingDir: dirpath,
	// }
	// g.ui.Confidentialf("Execute command %s", command)
	// result := command.Run()
	r := runcmdWrapperRequest{
		args:       args,
		workingDir: dirpath,
	}
	result := g.executor.exec(r)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("(%d) %v", result.ExitStatus(), result.Error())
		err = result.Error()
	}
	gitOutput := result.Stdout().String()
	g.ui.Title(dirpath)
	fmt.Println(string(gitOutput))
	return err
}

// Status executes `git status`
func (g *GitCommands) Status(dirpath string, untracked bool) error {
	var err error
	g.ui.Confidentialf("Status on %s", dirpath)
	// git, err := gitExecutablePath()
	// if err != nil {
	// 	return err
	// }
	// command := &runcmd.Command{
	// 	Exe:  git,
	// 	Args: statusArguments(dirpath, untracked),
	// }
	// g.ui.Confidentialf("Execute command %s", command)
	// result := command.Run()
	r := runcmdWrapperRequest{
		args: statusArguments(dirpath, untracked),
	}
	result := g.executor.exec(r)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("(%d) %v", result.ExitStatus(), result.Error())
		err = result.Error()
	}
	gitOutput := result.Stdout().String()
	if len(gitOutput) == 0 {
		g.ui.Confidentialf("%s unmodified", dirpath)
	} else {
		g.ui.Title(dirpath)
		fmt.Println(string(gitOutput))
	}
	return err
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

func gitExecutablePath() (string, error) {
	gitExecutable := environment.Which("git")
	if gitExecutable == "" {
		//ui.Errorf("git not found in path. exit\n")
		return "", fmt.Errorf(`git executable not found`)
	}
	return gitExecutable, nil
}
