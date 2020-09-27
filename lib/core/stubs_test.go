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
}

func (r runcmdStubResult) Success() bool {
	return r.success
}
func (r runcmdStubResult) Stderr() *bytes.Buffer {
	var b bytes.Buffer
	b.WriteString(r.stderr)
	return &b
}

// Stdout returns the underlying buffer with the contents of the output stream.
func (r runcmdStubResult) Stdout() *bytes.Buffer {
	var b bytes.Buffer
	b.WriteString(r.stdout)
	return &b
}

func (r runcmdStubResult) Error() error {
	return r.err
}
func (r runcmdStubResult) ExitStatus() int {
	return r.exitStatus
}
