package core

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"github.com/enr/clui"
)

type runcmdWrapperRequest struct {
	ctx        context.Context
	args       []string
	workingDir string
}

type runcmdWrapper interface {
	exec(r runcmdWrapperRequest) runcmdResult
}

type runcmdResult interface {
	Success() bool
	Stderr() *bytes.Buffer
	Stdout() *bytes.Buffer
	ExitStatus() int
	Error() error
}

func newGitExecutor(ui *clui.Clui) (runcmdWrapper, error) {
	git, err := gitExecutablePath()
	if err != nil {
		return defaultGitWrapper{}, err
	}
	return defaultGitWrapper{
		git: git,
		ui:  ui,
	}, nil
}

type defaultGitWrapper struct {
	git string
	ui  *clui.Clui
}

type execResult struct {
	stdout     bytes.Buffer
	stderr     bytes.Buffer
	exitStatus int
	err        error
}

func (r *execResult) Success() bool         { return r.err == nil }
func (r *execResult) Stdout() *bytes.Buffer { return &r.stdout }
func (r *execResult) Stderr() *bytes.Buffer { return &r.stderr }
func (r *execResult) ExitStatus() int       { return r.exitStatus }
func (r *execResult) Error() error          { return r.err }

func (g defaultGitWrapper) exec(r runcmdWrapperRequest) runcmdResult {
	ctx := r.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, g.git, r.args...)
	cmd.Dir = r.workingDir
	// If the context is cancelled and a child process holds the stdout/stderr
	// pipe open (e.g. ssh spawned by git), WaitDelay ensures cmd.Run() returns
	// after at most 5 s rather than hanging until the orphan exits.
	cmd.WaitDelay = 5 * time.Second

	result := &execResult{}
	cmd.Stdout = &result.stdout
	cmd.Stderr = &result.stderr

	g.ui.Confidentialf("Execute command %s %v in %s", g.git, r.args, r.workingDir)
	if err := cmd.Run(); err != nil {
		result.err = err
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.exitStatus = exitErr.ExitCode()
		} else {
			result.exitStatus = -1
		}
	}
	return result
}
