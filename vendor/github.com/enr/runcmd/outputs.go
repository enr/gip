package runcmd

import (
	"bytes"
)

// outputs contains streams for the "standard" output and "standard" error of a command.
type outputs struct {
	out *bytes.Buffer
	err *bytes.Buffer
}

// stderr returns the error stream as `*bytes.Buffer`.
func (s *outputs) stderr() *bytes.Buffer {
	return s.err
}

// stdout returns the output stream as `*bytes.Buffer`.
func (s *outputs) stdout() *bytes.Buffer {
	return s.out
}
