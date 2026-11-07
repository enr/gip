package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/enr/clui"
)

// CommandRunnerOption configures a CommandRunner.
type CommandRunnerOption func(*CommandRunner)

// WithSharedOutputRunner makes all display sections share mu and call beforeDisplay
// (while holding mu) before writing to the UI.
func WithSharedOutputRunner(mu *sync.Mutex, beforeDisplay func()) CommandRunnerOption {
	return func(r *CommandRunner) {
		r.outMu = mu
		r.beforeDisplay = beforeDisplay
	}
}

// CommandRunner executes arbitrary commands inside project directories.
type CommandRunner struct {
	ui            *clui.Clui
	outMu         *sync.Mutex
	beforeDisplay func()
}

// NewCommandRunner creates a CommandRunner.
func NewCommandRunner(ui *clui.Clui, opts ...CommandRunnerOption) *CommandRunner {
	r := &CommandRunner{
		ui:            ui,
		outMu:         &sync.Mutex{},
		beforeDisplay: func() {},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ExecResult is the outcome of a command run by CommandRunner.
type ExecResult struct {
	Stdout string
	Stderr string
	// ExitCode is the command's exit status; -1 when it did not exit on its
	// own (not started, killed by a timeout).
	ExitCode int
}

// Run executes name with args inside workingDir, synchronising output via an
// internal mutex. The result is returned even when the command fails.
func (r *CommandRunner) Run(ctx context.Context, workingDir, name string, args []string) (ExecResult, error) {
	if strings.HasPrefix(name, "-") {
		return ExecResult{ExitCode: -1}, fmt.Errorf("invalid command name: cannot start with '-'")
	}

	r.ui.Confidentialf("Execute %s %v in %s", name, args, workingDir)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workingDir
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := ExecResult{ExitCode: 0}
	if err != nil {
		res.ExitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.Exited() {
			res.ExitCode = exitErr.ExitCode()
		}
	}
	runErr := contextError(ctx, err)
	if runErr != nil && ctx.Err() != nil {
		res.ExitCode = -1
	}
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()

	r.outMu.Lock()
	r.beforeDisplay()
	r.ui.Title(workingDir)
	if stdout.Len() > 0 {
		r.ui.Lifecycle(res.Stdout)
	}
	if stderr.Len() > 0 {
		r.ui.Lifecycle(res.Stderr)
	}
	if runErr != nil {
		r.ui.Errorf("%s failed: %v", name, runErr)
	}
	r.outMu.Unlock()

	return res, runErr
}
