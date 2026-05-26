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
	results  []runcmdResult // returned in order, one per exec call; falls back to result
	result   runcmdResult   // returned when results is exhausted; nil means empty success
}

func (m *mockGitWrapper) exec(r runcmdWrapperRequest) runcmdResult {
	m.requests = append(m.requests, r)
	if len(m.results) > 0 {
		res := m.results[0]
		m.results = m.results[1:]
		return res
	}
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

func TestGitCommands_StatusInfo(t *testing.T) {
	porcelainSynced := `# branch.oid abc123
# branch.head main
# branch.upstream origin/main
# branch.ab +0 -0
`
	porcelainAhead := `# branch.oid abc123
# branch.head feature
# branch.upstream origin/feature
# branch.ab +2 -0
1 M. N... 100644 100644 100644 aaa bbb file.go
`
	porcelainBehind := `# branch.oid abc123
# branch.head main
# branch.upstream origin/main
# branch.ab +0 -3
`
	porcelainDiverged := `# branch.oid abc123
# branch.head main
# branch.upstream origin/main
# branch.ab +1 -2
? untracked.go
`
	porcelainNoRemote := `# branch.oid abc123
# branch.head local-only
`

	tests := []struct {
		name          string
		statusOutput  string
		stashOutput   string
		wantSync      BranchSyncStatus
		wantDirty     DirtyStatus
		expectedError bool
	}{
		{
			name:         "synced no changes",
			statusOutput: porcelainSynced,
			wantSync:     BranchSyncStatus{Ahead: 0, Behind: 0, NoRemote: false},
			wantDirty:    DirtyStatus{},
		},
		{
			name:         "ahead with staged file",
			statusOutput: porcelainAhead,
			wantSync:     BranchSyncStatus{Ahead: 2, Behind: 0, NoRemote: false},
			wantDirty:    DirtyStatus{Staged: true},
		},
		{
			name:         "behind no changes",
			statusOutput: porcelainBehind,
			wantSync:     BranchSyncStatus{Ahead: 0, Behind: 3, NoRemote: false},
			wantDirty:    DirtyStatus{},
		},
		{
			name:         "diverged with untracked",
			statusOutput: porcelainDiverged,
			wantSync:     BranchSyncStatus{Ahead: 1, Behind: 2, NoRemote: false},
			wantDirty:    DirtyStatus{Untracked: true},
		},
		{
			name:         "no remote",
			statusOutput: porcelainNoRemote,
			wantSync:     BranchSyncStatus{NoRemote: true},
			wantDirty:    DirtyStatus{},
		},
		{
			name:         "stash present",
			statusOutput: porcelainSynced,
			stashOutput:  "stash@{0}: WIP on main: abc fix\n",
			wantSync:     BranchSyncStatus{},
			wantDirty:    DirtyStatus{Stashed: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ui := clui.DefaultClui()
			mock := &mockGitWrapper{
				results: []runcmdResult{
					&runcmdStubResult{success: true, stdout: tc.statusOutput},
					&runcmdStubResult{success: true, stdout: tc.stashOutput},
				},
			}
			gitCmd := &GitCommands{ui: ui, executor: mock}

			sync, dirty, err := gitCmd.StatusInfo(context.Background(), "/tmp/repo")
			if tc.expectedError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sync != tc.wantSync {
				t.Errorf("sync: got %+v, want %+v", sync, tc.wantSync)
			}
			if dirty != tc.wantDirty {
				t.Errorf("dirty: got %+v, want %+v", dirty, tc.wantDirty)
			}
			if len(mock.requests) != 2 {
				t.Errorf("expected 2 git calls, got %d", len(mock.requests))
			}
		})
	}

	t.Run("injection prevention", func(t *testing.T) {
		ui := clui.DefaultClui()
		gitCmd := &GitCommands{ui: ui, executor: &mockGitWrapper{}}
		_, _, err := gitCmd.StatusInfo(context.Background(), "--repo")
		if err == nil {
			t.Fatal("expected error for dirpath starting with '-'")
		}
	})
}

func TestGitCommands_LastCommit(t *testing.T) {
	t.Run("returns subject and relative date", func(t *testing.T) {
		ui := clui.DefaultClui()
		mock := &mockGitWrapper{
			result: &runcmdStubResult{success: true, stdout: "fix: handle edge case\n2 days ago\n"},
		}
		gitCmd := &GitCommands{ui: ui, executor: mock}

		subject, relDate, err := gitCmd.LastCommit(context.Background(), "/tmp/repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if subject != "fix: handle edge case" {
			t.Errorf("subject: got %q, want %q", subject, "fix: handle edge case")
		}
		if relDate != "2 days ago" {
			t.Errorf("relDate: got %q, want %q", relDate, "2 days ago")
		}
		expectedArgs := []string{"log", "-1", "--format=%s%n%cr"}
		if !reflect.DeepEqual(mock.requests[0].args, expectedArgs) {
			t.Errorf("args: got %v, want %v", mock.requests[0].args, expectedArgs)
		}
	})

	t.Run("no commits returns placeholder", func(t *testing.T) {
		ui := clui.DefaultClui()
		mock := &mockGitWrapper{
			result: &runcmdStubResult{success: true, stdout: ""},
		}
		gitCmd := &GitCommands{ui: ui, executor: mock}

		subject, relDate, err := gitCmd.LastCommit(context.Background(), "/tmp/repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if subject != "(no commits)" {
			t.Errorf("got %q, want %q", subject, "(no commits)")
		}
		if relDate != "" {
			t.Errorf("expected empty relDate, got %q", relDate)
		}
	})

	t.Run("injection prevention", func(t *testing.T) {
		ui := clui.DefaultClui()
		gitCmd := &GitCommands{ui: ui, executor: &mockGitWrapper{}}
		_, _, err := gitCmd.LastCommit(context.Background(), "--repo")
		if err == nil {
			t.Fatal("expected error for dirpath starting with '-'")
		}
	})
}
