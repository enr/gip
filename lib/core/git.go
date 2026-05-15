package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/enr/clui"
	"github.com/enr/go-commons/environment"
)

// NewGit is the factory function for GitCommands
func NewGit(ui *clui.Clui) (*GitCommands, error) {
	executor, err := newGitExecutor(ui)
	if err != nil {
		return nil, err
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
	outputMu sync.Mutex
}

// Clone executes `git clone`
func (g *GitCommands) Clone(ctx context.Context, repourl string, dirpath string) error {
	var err error
	if strings.HasPrefix(repourl, "-") {
		return fmt.Errorf("invalid repourl: cannot start with '-'")
	}
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	g.ui.Confidentialf("Cloning %s to %s", repourl, dirpath)
	args := []string{
		"clone",
		"--",
		repourl,
		dirpath,
	}
	r := runcmdWrapperRequest{
		ctx:  ctx,
		args: args,
	}
	result := g.executor.exec(r)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("(%d) %v", result.ExitStatus(), result.Error())
		err = result.Error()
	}
	gitOutput := result.Stdout().String()
	g.outputMu.Lock()
	g.ui.Title(dirpath)
	g.ui.Lifecycle(gitOutput)
	g.outputMu.Unlock()
	return err
}

// Pull executes `git pull`
func (g *GitCommands) Pull(ctx context.Context, dirpath string) error {
	var err error
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	g.ui.Confidentialf("Pulling %s", dirpath)
	args := []string{
		"pull",
	}
	r := runcmdWrapperRequest{
		ctx:        ctx,
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
	g.outputMu.Lock()
	g.ui.Title(dirpath)
	g.ui.Lifecycle(gitOutput)
	g.outputMu.Unlock()
	return err
}

// Status executes `git status`
func (g *GitCommands) Status(ctx context.Context, dirpath string, untracked bool) error {
	var err error
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	g.ui.Confidentialf("Status on %s", dirpath)
	r := runcmdWrapperRequest{
		ctx:        ctx,
		args:       statusArguments(untracked),
		workingDir: dirpath,
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
		g.outputMu.Lock()
		g.ui.Title(dirpath)
		g.ui.Lifecycle(gitOutput)
		g.outputMu.Unlock()
	}
	return err
}

// Fetch executes `git fetch --all --prune`
func (g *GitCommands) Fetch(ctx context.Context, dirpath string) error {
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	g.ui.Confidentialf("Fetching %s", dirpath)
	r := runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"fetch", "--all", "--prune"},
		workingDir: dirpath,
	}
	result := g.executor.exec(r)
	var err error
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("(%d) %v", result.ExitStatus(), result.Error())
		err = result.Error()
	}
	gitOutput := result.Stdout().String()
	g.outputMu.Lock()
	g.ui.Title(dirpath)
	g.ui.Lifecycle(gitOutput)
	g.outputMu.Unlock()
	return err
}

// CurrentBranch returns the name of the current branch, or "(detached)" for a detached HEAD.
func (g *GitCommands) CurrentBranch(ctx context.Context, dirpath string) (string, error) {
	if strings.HasPrefix(dirpath, "-") {
		return "", fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	r := runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"rev-parse", "--abbrev-ref", "HEAD"},
		workingDir: dirpath,
	}
	result := g.executor.exec(r)
	if !result.Success() {
		return "", result.Error()
	}
	branch := strings.TrimSpace(result.Stdout().String())
	if branch == "HEAD" {
		branch = "(detached)"
	}
	return branch, nil
}

func statusArguments(untracked bool) []string {
	untrackedFlag := "=no"
	if untracked {
		untrackedFlag = ""
	}
	args := []string{
		"status",
		"--porcelain",
		fmt.Sprintf("--untracked-files%s", untrackedFlag),
	}
	return args
}

func gitExecutablePath() (string, error) {
	gitExecutable := environment.Which("git")
	if gitExecutable == "" {
		return "", fmt.Errorf(`git executable not found`)
	}
	return gitExecutable, nil
}
