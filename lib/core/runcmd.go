package core

import (
	"bytes"

	"github.com/enr/clui"
	"github.com/enr/runcmd"
)

type runcmdWrapperRequest struct {
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

func (g defaultGitWrapper) exec(r runcmdWrapperRequest) runcmdResult {
	command := &runcmd.Command{
		Exe:        g.git,
		Args:       r.args,
		WorkingDir: r.workingDir,
	}
	g.ui.Confidentialf("Execute command %s", command)
	return command.Run()
}
