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
}

func (m *mockGitWrapper) exec(r runcmdWrapperRequest) runcmdResult {
	m.requests = append(m.requests, r)
	return &runcmdStubResult{success: true}
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
