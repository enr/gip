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

// CommandRunner executes arbitrary commands inside project directories.
type CommandRunner struct {
	ui       *clui.Clui
	outputMu sync.Mutex
}

// NewCommandRunner creates a CommandRunner.
func NewCommandRunner(ui *clui.Clui) *CommandRunner {
	return &CommandRunner{ui: ui}
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

	r.outputMu.Lock()
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
	r.outputMu.Unlock()

	return runErr
}
