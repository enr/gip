package core

import (
	"bytes"
	"context"
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

// Run executes name with args inside workingDir, synchronising output via an internal mutex.
func (r *CommandRunner) Run(ctx context.Context, workingDir, name string, args []string) error {
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("invalid command name: cannot start with '-'")
	}

	r.ui.Confidentialf("Execute %s %v in %s", name, args, workingDir)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workingDir
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	r.outMu.Lock()
	r.beforeDisplay()
	r.ui.Title(workingDir)
	if stdout.Len() > 0 {
		r.ui.Lifecycle(stdout.String())
	}
	if stderr.Len() > 0 {
		r.ui.Lifecycle(stderr.String())
	}
	if runErr != nil {
		r.ui.Errorf("(%v)", runErr)
	}
	r.outMu.Unlock()

	return runErr
}
