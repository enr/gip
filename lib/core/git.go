package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/enr/clui"
	"github.com/enr/go-commons/environment"
)

// gitResultError builds an error that includes git's stderr output (which is
// where git writes its "fatal: ..." diagnostic messages).
func gitResultError(result runcmdResult) error {
	msg := strings.TrimSpace(result.Stderr().String())
	if msg == "" {
		return result.Error()
	}
	return fmt.Errorf("%w: %s", result.Error(), msg)
}

// GitOption configures a GitCommands instance.
type GitOption func(*GitCommands)

// WithSharedOutput makes all git display sections share mu and call beforeDisplay
// (while holding mu) before writing to the UI. This lets a caller serialise
// progress-bar updates with git output without holding the lock across subprocesses.
func WithSharedOutput(mu *sync.Mutex, beforeDisplay func()) GitOption {
	return func(g *GitCommands) {
		g.outMu = mu
		g.beforeDisplay = beforeDisplay
	}
}

// NewGit is the factory function for GitCommands
func NewGit(ui *clui.Clui, opts ...GitOption) (*GitCommands, error) {
	executor, err := newGitExecutor(ui)
	if err != nil {
		return nil, err
	}
	g := &GitCommands{
		ui:            ui,
		executor:      executor,
		outMu:         &sync.Mutex{},
		beforeDisplay: func() {},
	}
	for _, opt := range opts {
		opt(g)
	}
	return g, nil
}

// ensureOut guarantees outMu and beforeDisplay are initialised even when
// GitCommands is constructed directly (e.g., in tests via struct literals).
func (g *GitCommands) ensureOut() {
	if g.outMu == nil {
		g.outMu = &sync.Mutex{}
	}
	if g.beforeDisplay == nil {
		g.beforeDisplay = func() {}
	}
}

// GitCommands ...
type GitCommands struct {
	ui            *clui.Clui
	executor      runcmdWrapper
	outMu         *sync.Mutex
	beforeDisplay func()
}

// Clone executes `git clone`
func (g *GitCommands) Clone(ctx context.Context, repourl string, dirpath string) error {
	g.ensureOut()
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
		err = gitResultError(result)
	}
	g.outMu.Lock()
	g.beforeDisplay()
	g.ui.Title(dirpath)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("(%d) %s", result.ExitStatus(), strings.TrimSpace(result.Stderr().String()))
	}
	g.ui.Lifecycle(result.Stdout().String())
	g.outMu.Unlock()
	return err
}

// Pull executes `git pull`
func (g *GitCommands) Pull(ctx context.Context, dirpath string) error {
	g.ensureOut()
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
		err = gitResultError(result)
	}
	g.outMu.Lock()
	g.beforeDisplay()
	g.ui.Title(dirpath)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("(%d) %s", result.ExitStatus(), strings.TrimSpace(result.Stderr().String()))
	}
	g.ui.Lifecycle(result.Stdout().String())
	g.outMu.Unlock()
	return err
}

// Status executes `git status`
func (g *GitCommands) Status(ctx context.Context, dirpath string, untracked bool) error {
	g.ensureOut()
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
		err = gitResultError(result)
	}
	gitOutput := result.Stdout().String()
	if len(gitOutput) == 0 && result.Success() {
		g.ui.Confidentialf("%s unmodified", dirpath)
	} else {
		g.outMu.Lock()
		g.beforeDisplay()
		g.ui.Title(dirpath)
		if !result.Success() {
			g.ui.Errorf("Error executing Git in %s", dirpath)
			g.ui.Errorf("(%d) %s", result.ExitStatus(), strings.TrimSpace(result.Stderr().String()))
		}
		g.ui.Lifecycle(gitOutput)
		g.outMu.Unlock()
	}
	return err
}

// Fetch executes `git fetch --all --prune`
func (g *GitCommands) Fetch(ctx context.Context, dirpath string) error {
	g.ensureOut()
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
		err = gitResultError(result)
	}
	g.outMu.Lock()
	g.beforeDisplay()
	g.ui.Title(dirpath)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("(%d) %s", result.ExitStatus(), strings.TrimSpace(result.Stderr().String()))
	}
	g.ui.Lifecycle(result.Stdout().String())
	g.outMu.Unlock()
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
		return "", gitResultError(result)
	}
	branch := strings.TrimSpace(result.Stdout().String())
	if branch == "HEAD" {
		branch = "(detached)"
	}
	return branch, nil
}

// BranchSyncStatus holds the upstream delta for the checked-out branch.
type BranchSyncStatus struct {
	Ahead    int
	Behind   int
	NoRemote bool // true when no upstream tracking branch is configured
}

// SyncLabel returns a compact display string (e.g. "synced", "↑2", "↓3", "↑1↓2", "no-remote").
func (s BranchSyncStatus) SyncLabel() string {
	if s.NoRemote {
		return "no-remote"
	}
	if s.Ahead == 0 && s.Behind == 0 {
		return "synced"
	}
	if s.Ahead > 0 && s.Behind > 0 {
		return fmt.Sprintf("↑%d↓%d", s.Ahead, s.Behind)
	}
	if s.Ahead > 0 {
		return fmt.Sprintf("↑%d", s.Ahead)
	}
	return fmt.Sprintf("↓%d", s.Behind)
}

// DirtyStatus captures working-tree and index state.
type DirtyStatus struct {
	Staged    bool
	Unstaged  bool
	Untracked bool
	Stashed   bool
}

// Symbols returns compact status indicators (+*?$), or "—" when clean.
func (d DirtyStatus) Symbols() string {
	var b strings.Builder
	if d.Staged {
		b.WriteByte('+')
	}
	if d.Unstaged {
		b.WriteByte('*')
	}
	if d.Untracked {
		b.WriteByte('?')
	}
	if d.Stashed {
		b.WriteByte('$')
	}
	if b.Len() == 0 {
		return "—"
	}
	return b.String()
}

// IsDirty reports whether the working tree or index has any uncommitted change.
func (d DirtyStatus) IsDirty() bool {
	return d.Staged || d.Unstaged || d.Untracked
}

// StatusInfo runs "git status --porcelain=v2 --branch" and "git stash list" to
// collect upstream delta and working-tree indicators in two calls.
func (g *GitCommands) StatusInfo(ctx context.Context, dirpath string) (BranchSyncStatus, DirtyStatus, error) {
	if strings.HasPrefix(dirpath, "-") {
		return BranchSyncStatus{}, DirtyStatus{}, fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	r := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"status", "--porcelain=v2", "--branch"},
		workingDir: dirpath,
	})
	if !r.Success() {
		return BranchSyncStatus{}, DirtyStatus{}, gitResultError(r)
	}

	var sync BranchSyncStatus
	var dirty DirtyStatus
	sync.NoRemote = true

	for _, line := range strings.Split(r.Stdout().String(), "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.ab "):
			sync.NoRemote = false
			fmt.Sscanf(strings.TrimPrefix(line, "# branch.ab "), "+%d -%d", &sync.Ahead, &sync.Behind)
		case strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 "):
			if len(line) >= 4 {
				if line[2] != '.' {
					dirty.Staged = true
				}
				if line[3] != '.' {
					dirty.Unstaged = true
				}
			}
		case strings.HasPrefix(line, "u "):
			// unmerged (conflict) — treat as staged
			dirty.Staged = true
		case strings.HasPrefix(line, "? "):
			dirty.Untracked = true
		}
	}

	stash := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"stash", "list"},
		workingDir: dirpath,
	})
	if stash.Success() && strings.TrimSpace(stash.Stdout().String()) != "" {
		dirty.Stashed = true
	}

	return sync, dirty, nil
}

// LastCommit returns the subject line and relative date of the most recent commit.
func (g *GitCommands) LastCommit(ctx context.Context, dirpath string) (subject, relDate string, err error) {
	if strings.HasPrefix(dirpath, "-") {
		return "", "", fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	r := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"log", "-1", "--format=%s%n%cr"},
		workingDir: dirpath,
	})
	if !r.Success() {
		return "", "", gitResultError(r)
	}
	out := strings.TrimSpace(r.Stdout().String())
	if out == "" {
		return "(no commits)", "", nil
	}
	parts := strings.SplitN(out, "\n", 2)
	subject = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		relDate = strings.TrimSpace(parts[1])
	}
	return subject, relDate, nil
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
