package core

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/enr/clui"
)

var errGeneric = fmt.Errorf(`test error`)

type testGitWrapper struct {
}

func (g testGitWrapper) exec(r runcmdWrapperRequest) runcmdResult {
	return &runcmdStubResult{
		stdout:     `stdout`,
		stderr:     `stderr`,
		success:    false,
		exitStatus: 4,
		err:        errGeneric,
	}
}

type testGitWrapperSuccess struct{}

func (g testGitWrapperSuccess) exec(r runcmdWrapperRequest) runcmdResult {
	return &runcmdStubResult{
		stdout:     `stdout`,
		stderr:     ``,
		success:    true,
		exitStatus: 0,
		err:        nil,
	}
}

func TestPullError(t *testing.T) {
	sut := &GitCommands{
		ui:       clui.DefaultClui(),
		executor: testGitWrapper{},
	}
	err := sut.Pull(context.Background(), "dirpath")
	if err == nil {
		t.Errorf("Expected error in pull but got NIL")
	}
	if !errors.Is(err, errGeneric) {
		t.Errorf(`Expectederror %v but got %v`, errGeneric, err)
	}
}

func TestStatusError(t *testing.T) {
	sut := &GitCommands{
		ui:       clui.DefaultClui(),
		executor: testGitWrapper{},
	}
	err := sut.Status(context.Background(), "dirpath", true)
	if err == nil {
		t.Errorf("Expected error in status but got NIL")
	}
	if !errors.Is(err, errGeneric) {
		t.Errorf(`Expectederror %v but got %v`, errGeneric, err)
	}
}

func TestCloneError(t *testing.T) {
	sut := &GitCommands{
		ui:       clui.DefaultClui(),
		executor: testGitWrapper{},
	}
	err := sut.Clone(context.Background(), "repourl", "dirpath")
	if err == nil {
		t.Errorf("Expected error in clone but got NIL")
	}
	if !errors.Is(err, errGeneric) {
		t.Errorf(`Expectederror %v but got %v`, errGeneric, err)
	}
}

func TestPullSuccess(t *testing.T) {
	sut := &GitCommands{
		ui:       clui.DefaultClui(),
		executor: testGitWrapperSuccess{},
	}
	err := sut.Pull(context.Background(), "dirpath")
	if err != nil {
		t.Errorf("Expected no error in pull but got %v", err)
	}
}

func TestStatusSuccess(t *testing.T) {
	sut := &GitCommands{
		ui:       clui.DefaultClui(),
		executor: testGitWrapperSuccess{},
	}
	for _, untracked := range []bool{true, false} {
		err := sut.Status(context.Background(), "dirpath", untracked)
		if err != nil {
			t.Errorf("Expected no error in status (untracked=%v) but got %v", untracked, err)
		}
	}
}

func TestStatusArguments(t *testing.T) {
	withUntracked := statusArguments(true)
	withoutUntracked := statusArguments(false)
	for _, arg := range withUntracked {
		if arg == "--untracked-files=no" {
			t.Errorf("Expected untracked-files enabled but got =no flag")
		}
	}
	found := false
	for _, arg := range withoutUntracked {
		if arg == "--untracked-files=no" {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected --untracked-files=no when untracked=false")
	}
}

func TestCloneSuccess(t *testing.T) {
	sut := &GitCommands{
		ui:       clui.DefaultClui(),
		executor: testGitWrapperSuccess{},
	}
	err := sut.Clone(context.Background(), "repourl", "dirpath")
	if err != nil {
		t.Errorf("Expected no error in clone but got %v", err)
	}
}
