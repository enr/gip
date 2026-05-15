package core

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/enr/clui"
)

type mockGitWrapper struct {
	requests []runcmdWrapperRequest
	result   runcmdResult // if nil, defaults to a successful empty result
}

func (m *mockGitWrapper) exec(r runcmdWrapperRequest) runcmdResult {
	m.requests = append(m.requests, r)
	if m.result != nil {
		return m.result
	}
	return &runcmdStubResult{success: true}
}

func TestNewGit_ReturnsNilOnError(t *testing.T) {
	t.Setenv("PATH", "")
	ui := clui.DefaultClui()
	git, err := NewGit(ui)
	if err == nil {
		t.Fatal("expected error when git is not found, got nil")
	}
	if git != nil {
		t.Fatalf("expected nil *GitCommands on error, got %v", git)
	}
}

func TestGitCommands_Clone(t *testing.T) {
	tests := []struct {
		name          string
		repourl       string
		dirpath       string
		expectedArgs  []string
		expectedError bool
	}{
		{
			name:          "normal clone",
			repourl:       "https://github.com/enr/gip.git",
			dirpath:       "/tmp/gip",
			expectedArgs:  []string{"clone", "--", "https://github.com/enr/gip.git", "/tmp/gip"},
			expectedError: false,
		},
		{
			name:          "option injection repourl",
			repourl:       "--upload-pack=/tmp/x.sh",
			dirpath:       "/tmp/gip",
			expectedError: true,
		},
		{
			name:          "option injection dirpath",
			repourl:       "https://github.com/enr/gip.git",
			dirpath:       "--work-tree=/tmp/x",
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ui := clui.DefaultClui()
			mock := &mockGitWrapper{}
			gitCmd := &GitCommands{
				ui:       ui,
				executor: mock,
			}

			if !tc.expectedError {
				os.RemoveAll(tc.dirpath) // clean up before
			}

			err := gitCmd.Clone(context.Background(), tc.repourl, tc.dirpath)
			if tc.expectedError {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if len(mock.requests) != 1 {
					t.Fatalf("Expected 1 execution, got %d", len(mock.requests))
				}
				if !reflect.DeepEqual(mock.requests[0].args, tc.expectedArgs) {
					t.Fatalf("Expected args %v, got %v", tc.expectedArgs, mock.requests[0].args)
				}
			}
		})
	}
}

func TestGitCommands_Pull(t *testing.T) {
	ui := clui.DefaultClui()
	mock := &mockGitWrapper{}
	gitCmd := &GitCommands{
		ui:       ui,
		executor: mock,
	}

	err := gitCmd.Pull(context.Background(), "/tmp/myrepo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(mock.requests) != 1 {
		t.Fatalf("Expected 1 execution, got %d", len(mock.requests))
	}
	expectedArgs := []string{"pull"}
	if !reflect.DeepEqual(mock.requests[0].args, expectedArgs) {
		t.Fatalf("Expected args %v, got %v", expectedArgs, mock.requests[0].args)
	}
	if mock.requests[0].workingDir != "/tmp/myrepo" {
		t.Fatalf("Expected workingDir /tmp/myrepo, got %v", mock.requests[0].workingDir)
	}

	// test option injection dirpath
	err = gitCmd.Pull(context.Background(), "--myrepo")
	if err == nil {
		t.Fatalf("Expected error for dirpath starting with -")
	}
}

func TestGitCommands_Fetch(t *testing.T) {
	ui := clui.DefaultClui()
	mock := &mockGitWrapper{}
	gitCmd := &GitCommands{
		ui:       ui,
		executor: mock,
	}

	err := gitCmd.Fetch(context.Background(), "/tmp/myrepo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(mock.requests) != 1 {
		t.Fatalf("Expected 1 execution, got %d", len(mock.requests))
	}
	expectedArgs := []string{"fetch", "--all", "--prune"}
	if !reflect.DeepEqual(mock.requests[0].args, expectedArgs) {
		t.Fatalf("Expected args %v, got %v", expectedArgs, mock.requests[0].args)
	}
	if mock.requests[0].workingDir != "/tmp/myrepo" {
		t.Fatalf("Expected workingDir /tmp/myrepo, got %v", mock.requests[0].workingDir)
	}

	// option injection prevention
	err = gitCmd.Fetch(context.Background(), "--myrepo")
	if err == nil {
		t.Fatal("Expected error for dirpath starting with -")
	}
}

func TestGitCommands_CurrentBranch(t *testing.T) {
	t.Run("returns branch name", func(t *testing.T) {
		ui := clui.DefaultClui()
		mock := &mockGitWrapper{
			result: &runcmdStubResult{success: true, stdout: "main\n"},
		}
		gitCmd := &GitCommands{ui: ui, executor: mock}

		branch, err := gitCmd.CurrentBranch(context.Background(), "/tmp/myrepo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if branch != "main" {
			t.Fatalf("expected 'main', got %q", branch)
		}
		expectedArgs := []string{"rev-parse", "--abbrev-ref", "HEAD"}
		if !reflect.DeepEqual(mock.requests[0].args, expectedArgs) {
			t.Fatalf("expected args %v, got %v", expectedArgs, mock.requests[0].args)
		}
	})

	t.Run("detached HEAD becomes (detached)", func(t *testing.T) {
		ui := clui.DefaultClui()
		mock := &mockGitWrapper{
			result: &runcmdStubResult{success: true, stdout: "HEAD\n"},
		}
		gitCmd := &GitCommands{ui: ui, executor: mock}

		branch, err := gitCmd.CurrentBranch(context.Background(), "/tmp/myrepo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if branch != "(detached)" {
			t.Fatalf("expected '(detached)', got %q", branch)
		}
	})

	t.Run("injection prevention", func(t *testing.T) {
		ui := clui.DefaultClui()
		mock := &mockGitWrapper{}
		gitCmd := &GitCommands{ui: ui, executor: mock}

		_, err := gitCmd.CurrentBranch(context.Background(), "--myrepo")
		if err == nil {
			t.Fatal("expected error for dirpath starting with '-'")
		}
	})
}

func TestGitCommands_Status(t *testing.T) {
	ui := clui.DefaultClui()
	mock := &mockGitWrapper{}
	gitCmd := &GitCommands{
		ui:       ui,
		executor: mock,
	}

	err := gitCmd.Status(context.Background(), "/tmp/myrepo", false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(mock.requests) != 1 {
		t.Fatalf("Expected 1 execution, got %d", len(mock.requests))
	}
	expectedArgs := []string{"status", "--porcelain", "--untracked-files=no"}
	if !reflect.DeepEqual(mock.requests[0].args, expectedArgs) {
		t.Fatalf("Expected args %v, got %v", expectedArgs, mock.requests[0].args)
	}
	if mock.requests[0].workingDir != "/tmp/myrepo" {
		t.Fatalf("Expected workingDir /tmp/myrepo, got %v", mock.requests[0].workingDir)
	}

	// test option injection dirpath
	err = gitCmd.Status(context.Background(), "--myrepo", false)
	if err == nil {
		t.Fatalf("Expected error for dirpath starting with -")
	}
}
