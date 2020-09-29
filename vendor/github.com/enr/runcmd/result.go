package runcmd

import (
	"bytes"
	"fmt"
)

// ExecResult is the result of the completed command execution.
type ExecResult struct {
	fullCommand string
	outputs     *outputs
	err         error
}

// Success true if command completed with no error.
func (r *ExecResult) Success() bool {
	return r.err == nil && r.ExitStatus() == 0
}

// ExitStatus is the command exit.
func (r *ExecResult) ExitStatus() int {
	return getExitStatus(r.err)
}

func (r *ExecResult) String() string {
	status := "error"
	if r.Success() {
		status = "success"
	}
	return fmt.Sprintf("Command `%s` %s (%d)", r.fullCommand, status, r.ExitStatus())
}

// Stderr returns the underlying buffer with the contents of the error stream.
func (r *ExecResult) Stderr() *bytes.Buffer {
	return r.outputs.stderr()
}

// Stdout returns the underlying buffer with the contents of the output stream.
func (r *ExecResult) Stdout() *bytes.Buffer {
	return r.outputs.stdout()
}

func (r *ExecResult) Error() error {
	return r.err
}
