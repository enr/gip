package core

import (
	"fmt"
	"testing"

	"github.com/enr/clui"
)

var errGeneric = fmt.Errorf(`test error`)

type testGitWrapper struct {
}

func (g testGitWrapper) exec(r runcmdWrapperRequest) runcmdResult {
	return runcmdStubResult{
		stdout:     `stdout`,
		stderr:     `stderr`,
		success:    false,
		exitStatus: 4,
		err:        errGeneric,
	}
}

func TestPullError(t *testing.T) {
	sut := &GitCommands{
		ui:       clui.DefaultClui(),
		executor: testGitWrapper{},
	}
	err := sut.Pull("dirpath")
	if err == nil {
		t.Errorf("Expected error in pull but got NIL")
	}
	if err != errGeneric {
		t.Errorf(`Expectederror %v but got %v`, errGeneric, err)
	}
}

func TestStatusError(t *testing.T) {
	sut := &GitCommands{
		ui:       clui.DefaultClui(),
		executor: testGitWrapper{},
	}
	err := sut.Status("dirpath", true)
	if err == nil {
		t.Errorf("Expected error in pull but got NIL")
	}
	if err != errGeneric {
		t.Errorf(`Expectederror %v but got %v`, errGeneric, err)
	}
}

func TestCloneError(t *testing.T) {
	sut := &GitCommands{
		ui:       clui.DefaultClui(),
		executor: testGitWrapper{},
	}
	err := sut.Clone("repourl", "dirpath")
	if err == nil {
		t.Errorf("Expected error in pull but got NIL")
	}
	if err != errGeneric {
		t.Errorf(`Expectederror %v but got %v`, errGeneric, err)
	}
}
