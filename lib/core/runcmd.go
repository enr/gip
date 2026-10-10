package core

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/enr/clui"
)

type runcmdWrapperRequest struct {
	ctx        context.Context
	args       []string
	workingDir string
	// remote marks commands that may contact a remote (clone, pull, fetch):
	// only those need the non-interactive ssh setup.
	remote bool
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
	// Leaving Stdin nil makes os/exec connect the child to os.DevNull and close
	// it for us; opening /dev/null by hand here would leak a descriptor per call.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if r.remote {
		if sshCommand := g.sshCommand(ctx, r.workingDir); sshCommand != "" {
			cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND="+sshCommand)
		}
	}

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

// sshCommand returns the value for GIT_SSH_COMMAND that forces non-interactive
// ssh while preserving the user's own ssh setup, or "" to leave git's choice
// untouched. GIT_SSH_COMMAND overrides both GIT_SSH and core.sshCommand, so
// setting it blindly would discard a custom identity file, jump host or plink.
func (g defaultGitWrapper) sshCommand(ctx context.Context, workingDir string) string {
	if existing := os.Getenv("GIT_SSH_COMMAND"); existing != "" {
		return existing + " -o BatchMode=yes"
	}
	if os.Getenv("GIT_SSH") != "" {
		// GIT_SSH names a program (often plink), not a command line: options
		// cannot be appended safely, so let git use it as configured.
		return ""
	}
	cmd := exec.CommandContext(ctx, g.git, "config", "--get", "core.sshCommand")
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.WaitDelay = 5 * time.Second
	// Exit status 1 means the key is not set: fall back to plain ssh.
	if out, err := cmd.Output(); err == nil {
		if configured := strings.TrimSpace(string(out)); configured != "" {
			return configured + " -o BatchMode=yes"
		}
	}
	return "ssh -o BatchMode=yes"
}
