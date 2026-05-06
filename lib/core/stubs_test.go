package core

import (
	"bytes"
)

type runcmdStubResult struct {
	success    bool
	stdout     string
	stderr     string
	err        error
	exitStatus int
	stdoutBuf  *bytes.Buffer
	stderrBuf  *bytes.Buffer
}

func (r *runcmdStubResult) Success() bool {
	return r.success
}
func (r *runcmdStubResult) Stderr() *bytes.Buffer {
	if r.stderrBuf == nil {
		r.stderrBuf = bytes.NewBufferString(r.stderr)
	}
	return r.stderrBuf
}

// Stdout returns the underlying buffer with the contents of the output stream.
func (r *runcmdStubResult) Stdout() *bytes.Buffer {
	if r.stdoutBuf == nil {
		r.stdoutBuf = bytes.NewBufferString(r.stdout)
	}
	return r.stdoutBuf
}

func (r *runcmdStubResult) Error() error {
	return r.err
}
func (r *runcmdStubResult) ExitStatus() int {
	return r.exitStatus
}
