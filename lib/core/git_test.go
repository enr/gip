package core

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
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
	expectedArgs := append(append([]string{}, statusConfig...), "status", "--porcelain", "--untracked-files=no")
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
			wantSync:     BranchSyncStatus{Branch: "main", Ahead: 0, Behind: 0, NoRemote: false},
			wantDirty:    DirtyStatus{},
		},
		{
			name:         "ahead with staged file",
			statusOutput: porcelainAhead,
			wantSync:     BranchSyncStatus{Branch: "feature", Ahead: 2, Behind: 0, NoRemote: false},
			wantDirty:    DirtyStatus{Staged: true},
		},
		{
			name:         "behind no changes",
			statusOutput: porcelainBehind,
			wantSync:     BranchSyncStatus{Branch: "main", Ahead: 0, Behind: 3, NoRemote: false},
			wantDirty:    DirtyStatus{},
		},
		{
			name:         "diverged with untracked",
			statusOutput: porcelainDiverged,
			wantSync:     BranchSyncStatus{Branch: "main", Ahead: 1, Behind: 2, NoRemote: false},
			wantDirty:    DirtyStatus{Untracked: true},
		},
		{
			name:         "no remote",
			statusOutput: porcelainNoRemote,
			wantSync:     BranchSyncStatus{Branch: "local-only", NoRemote: true},
			wantDirty:    DirtyStatus{},
		},
		{
			name:         "stash present",
			statusOutput: porcelainSynced,
			stashOutput:  "stash@{0}: WIP on main: abc fix\n",
			wantSync:     BranchSyncStatus{Branch: "main"},
			wantDirty:    DirtyStatus{Stashed: true},
		},
		{
			name:         "detached head",
			statusOutput: "# branch.oid abc\n# branch.head (detached)\n",
			wantSync:     BranchSyncStatus{Branch: "(detached)", NoRemote: true},
			wantDirty:    DirtyStatus{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ui := clui.DefaultClui()
			mock := &mockGitWrapper{
				results: []runcmdResult{
					// A git without "# stash" in porcelain v2: stash list is run.
					&runcmdStubResult{success: true, stdout: "git version 2.34.1\n"},
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
			if len(mock.requests) != 3 {
				t.Errorf("expected 3 git calls (version, status, stash list), got %d", len(mock.requests))
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

func TestGitCommands_BranchInfos(t *testing.T) {
	out := "dev\torigin/dev\torigin\tahead 1\t \trefs/heads/dev\n" +
		"rel\torigin/rel-1.x\torigin\tbehind 2\t \trefs/heads/rel-1.x\n" +
		"main\torigin/main\torigin\t\t*\trefs/heads/main\n" +
		"gone\torigin/gone\torigin\tgone\t \trefs/heads/gone\n" +
		"local\t\t\t\t \t\n"
	mock := &mockGitWrapper{result: &runcmdStubResult{success: true, stdout: out}}
	g := &GitCommands{ui: clui.DefaultClui(), executor: mock}

	infos, err := g.BranchInfos(context.Background(), "/tmp/r", []string{"dev", "rel", "main", "gone", "local", "missing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []BranchInfo{
		{Name: "dev", Exists: true, Remote: "origin", RemoteRef: "refs/heads/dev", Ahead: 1},
		{Name: "rel", Exists: true, Remote: "origin", RemoteRef: "refs/heads/rel-1.x", Behind: 2},
		{Name: "main", Exists: true, Remote: "origin", RemoteRef: "refs/heads/main", CheckedOut: true},
		{Name: "gone", Exists: true, Remote: "origin", RemoteRef: "refs/heads/gone", NoRemote: true},
		{Name: "local", Exists: true, NoRemote: true},
		{Name: "missing"},
	}
	if !reflect.DeepEqual(infos, want) {
		t.Fatalf("got %+v, want %+v", infos, want)
	}
	if got := mock.requests[0].args[len(mock.requests[0].args)-1]; got != "refs/heads/missing" {
		t.Fatalf("expected refs/heads/ patterns, last arg %q", got)
	}

	if _, err := g.BranchInfos(context.Background(), "/tmp/r", []string{"--evil"}); err == nil {
		t.Fatal("expected error for branch starting with '-'")
	}
	if _, err := g.BranchInfos(context.Background(), "--r", []string{"a"}); err == nil {
		t.Fatal("expected error for dirpath starting with '-'")
	}
}

func TestGitCommands_FastForwardBranch(t *testing.T) {
	mock := &mockGitWrapper{}
	g := &GitCommands{ui: clui.DefaultClui(), executor: mock}
	if err := g.FastForwardBranch(context.Background(), "/tmp/r", "origin", "", "rel"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"fetch", "origin", "refs/heads/rel:refs/heads/rel"}
	if !reflect.DeepEqual(mock.requests[0].args, want) {
		t.Fatalf("got args %v, want %v", mock.requests[0].args, want)
	}
	if err := g.FastForwardBranch(context.Background(), "/tmp/r", "origin", "refs/heads/rel-1.x", "rel"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want = []string{"fetch", "origin", "refs/heads/rel-1.x:refs/heads/rel"}
	if !reflect.DeepEqual(mock.requests[1].args, want) {
		t.Fatalf("got args %v, want %v", mock.requests[1].args, want)
	}
	if err := g.FastForwardBranch(context.Background(), "/tmp/r", "-x", "", "rel"); err == nil {
		t.Fatal("expected error for remote starting with '-'")
	}
	if err := g.FastForwardBranch(context.Background(), "/tmp/r", "origin", "", "-x"); err == nil {
		t.Fatal("expected error for branch starting with '-'")
	}
}

func TestParseDescribeLong(t *testing.T) {
	cases := []struct {
		in    string
		tag   string
		count int
		ok    bool
	}{
		{"v1.2.0-5-gabc1234\n", "v1.2.0", 5, true},
		{"v1.0-rc-1-0-g9635036", "v1.0-rc-1", 0, true},
		{"release-2024-12-3-gdeadbee", "release-2024-12", 3, true},
		{"garbage", "", 0, false},
		{"v1-x-gabc", "", 0, false},
	}
	for _, c := range cases {
		tag, count, err := parseDescribeLong(c.in)
		if (err == nil) != c.ok {
			t.Errorf("%q: err=%v, want ok=%v", c.in, err, c.ok)
			continue
		}
		if c.ok && (tag != c.tag || count != c.count) {
			t.Errorf("%q: got (%q, %d), want (%q, %d)", c.in, tag, count, c.tag, c.count)
		}
	}
}

var latestTagNoTag = &runcmdStubResult{success: false, err: errors.New("exit status 128"), exitStatus: 128, stderr: "fatal: No names found, cannot describe anything."}

func latestTag(t *testing.T, mock *mockGitWrapper, match string) TagInfo {
	t.Helper()
	info, err := (&GitCommands{ui: clui.DefaultClui(), executor: mock}).LatestTag(context.Background(), "/tmp/repo", match)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return info
}

func TestGitCommands_LatestTag_CommitsAfterTag(t *testing.T) {
	mock := &mockGitWrapper{results: []runcmdResult{
		&runcmdStubResult{success: true, stdout: "v1.2.0-5-gabc1234\n"},
		&runcmdStubResult{success: true, stdout: "3 weeks ago\n"},
	}}
	info := latestTag(t, mock, "v*")
	want := TagInfo{Tag: "v1.2.0", CommitsSince: 5, TagDate: "3 weeks ago"}
	if info != want || !info.Unreleased() {
		t.Errorf("got %+v (unreleased=%v), want %+v", info, info.Unreleased(), want)
	}
	if got := mock.requests[0].args; !reflect.DeepEqual(got, []string{"describe", "--tags", "--long", "--match=v*"}) {
		t.Errorf("describe args: %v", got)
	}
	if got := mock.requests[1].args; !reflect.DeepEqual(got, []string{"for-each-ref", "--format=%(creatordate:relative)", "refs/tags/v1.2.0"}) {
		t.Errorf("date args: %v", got)
	}
}

func TestGitCommands_LatestTag_HeadOnTag(t *testing.T) {
	mock := &mockGitWrapper{results: []runcmdResult{&runcmdStubResult{success: true, stdout: "v2.0.0-0-gabc1234\n"}}}
	info := latestTag(t, mock, "")
	if info.Tag != "v2.0.0" || info.CommitsSince != 0 || info.Unreleased() {
		t.Errorf("got %+v", info)
	}
	if got := mock.requests[0].args; !reflect.DeepEqual(got, []string{"describe", "--tags", "--long"}) {
		t.Errorf("describe args: %v", got)
	}
}

func TestGitCommands_LatestTag_NoTag(t *testing.T) {
	mock := &mockGitWrapper{results: []runcmdResult{latestTagNoTag, &runcmdStubResult{success: true, stdout: "42\n"}}}
	info := latestTag(t, mock, "")
	if info.Tag != "" || info.CommitsSince != 42 || !info.Unreleased() {
		t.Errorf("got %+v", info)
	}
}

func TestGitCommands_LatestTag_EmptyRepository(t *testing.T) {
	fail := &runcmdStubResult{success: false, err: errors.New("exit status 128"), exitStatus: 128}
	info := latestTag(t, &mockGitWrapper{results: []runcmdResult{latestTagNoTag, fail, fail}}, "")
	if !info.NoCommits || info.Unreleased() {
		t.Errorf("got %+v", info)
	}
}

func TestGitCommands_LatestTag_Errors(t *testing.T) {
	ui := clui.DefaultClui()
	mock := &mockGitWrapper{result: &runcmdStubResult{success: false, err: errors.New("exit status 128"), stderr: "fatal: not a git repository"}}
	_, err := (&GitCommands{ui: ui, executor: mock}).LatestTag(context.Background(), "/tmp/repo", "")
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected git error, got %v", err)
	}
	if _, err := (&GitCommands{ui: ui, executor: &mockGitWrapper{}}).LatestTag(context.Background(), "--repo", ""); err == nil {
		t.Error("expected error for dirpath starting with '-'")
	}
}

func TestGitCommands_RemoteURLs(t *testing.T) {
	out := "remote.origin.url git@github.com:enr/gip.git\nremote.my.fork.url https://example.com/x.git\n"
	mock := &mockGitWrapper{result: &runcmdStubResult{success: true, stdout: out}}
	g := &GitCommands{ui: clui.DefaultClui(), executor: mock}

	urls, err := g.RemoteURLs(context.Background(), "/tmp/r")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"origin": "git@github.com:enr/gip.git", "my.fork": "https://example.com/x.git"}
	if !reflect.DeepEqual(urls, want) {
		t.Fatalf("got %v, want %v", urls, want)
	}

	// exit status 1 without stderr: no remote configured
	mock = &mockGitWrapper{result: &runcmdStubResult{success: false, exitStatus: 1, err: errors.New("exit status 1")}}
	g = &GitCommands{ui: clui.DefaultClui(), executor: mock}
	urls, err = g.RemoteURLs(context.Background(), "/tmp/r")
	if err != nil || len(urls) != 0 {
		t.Fatalf("expected no remotes and no error, got %v, %v", urls, err)
	}

	mock = &mockGitWrapper{result: &runcmdStubResult{success: false, exitStatus: 128, stderr: "fatal: not a git repository", err: errors.New("exit status 128")}}
	g = &GitCommands{ui: clui.DefaultClui(), executor: mock}
	if _, err = g.RemoteURLs(context.Background(), "/tmp/r"); err == nil {
		t.Fatal("expected error outside a repository")
	}

	if _, err = g.RemoteURLs(context.Background(), "--r"); err == nil {
		t.Fatal("expected error for dirpath starting with '-'")
	}
}

func TestGitCommands_LsRemote(t *testing.T) {
	mock := &mockGitWrapper{}
	g := &GitCommands{ui: clui.DefaultClui(), executor: mock}
	if err := g.LsRemote(context.Background(), "/tmp/r", "https://example.com/x.git"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := mock.requests[0]
	if !reflect.DeepEqual(req.args, []string{"ls-remote", "https://example.com/x.git", "HEAD"}) || !req.remote {
		t.Fatalf("unexpected request %+v", req)
	}

	mock = &mockGitWrapper{result: &runcmdStubResult{success: false, stderr: "fatal: repository not found", err: errors.New("exit status 128")}}
	g = &GitCommands{ui: clui.DefaultClui(), executor: mock}
	err := g.LsRemote(context.Background(), "", "https://example.com/gone.git")
	if err == nil || !strings.Contains(err.Error(), "repository not found") {
		t.Fatalf("expected git's message in the error, got %v", err)
	}

	if err := g.LsRemote(context.Background(), "", "--upload-pack=evil"); err == nil {
		t.Fatal("expected error for URL starting with '-'")
	}
}

func TestGitVersionAtLeast(t *testing.T) {
	tests := []struct {
		output string
		want   bool
	}{
		{"git version 2.35.0\n", true},
		{"git version 2.43.0", true},
		{"git version 3.0.0", true},
		{"git version 2.41.0.windows.1", true},
		{"git version 2.39.3 (Apple Git-146)", true},
		{"git version 2.34.1", false},
		{"git version 1.99.0", false},
		{"", false},
		{"garbage", false},
	}
	for _, tc := range tests {
		if got := gitVersionAtLeast(tc.output, 2, 35); got != tc.want {
			t.Errorf("gitVersionAtLeast(%q) = %v, want %v", tc.output, got, tc.want)
		}
	}
}

// TestStatusInfoShowStash verifies that with git >= 2.35 the stash comes from
// the "# stash N" line of status, with no "git stash list" call.
func TestStatusInfoShowStash(t *testing.T) {
	synced := "# branch.oid abc123\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +0 -0\n"
	mock := &mockGitWrapper{
		results: []runcmdResult{
			&runcmdStubResult{success: true, stdout: "git version 2.43.0\n"},
			&runcmdStubResult{success: true, stdout: synced + "# stash 2\n"},
			&runcmdStubResult{success: true, stdout: synced},
		},
	}
	gitCmd := &GitCommands{ui: clui.DefaultClui(), executor: mock}
	_, dirty, err := gitCmd.StatusInfo(context.Background(), "/tmp/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dirty.Stashed {
		t.Errorf("expected Stashed from the # stash line, got %+v", dirty)
	}
	// The version is asked once: a second call is status only.
	_, dirty, _ = gitCmd.StatusInfo(context.Background(), "/tmp/repo")
	if dirty.Stashed {
		t.Errorf("expected no stash without the # stash line, got %+v", dirty)
	}
	if len(mock.requests) != 3 {
		t.Fatalf("expected 3 git calls (version, status, status), got %d", len(mock.requests))
	}
	want := append(append([]string{}, statusConfig...), "status", "--porcelain=v2", "--branch", "--show-stash")
	if !reflect.DeepEqual(mock.requests[1].args, want) {
		t.Errorf("args: got %v, want %v", mock.requests[1].args, want)
	}
}

func TestGitCommands_FetchTags(t *testing.T) {
	ui := clui.DefaultClui()
	mock := &mockGitWrapper{}
	if err := (&GitCommands{ui: ui, executor: mock}).FetchTags(context.Background(), "/tmp/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := mock.requests[0]; !reflect.DeepEqual(got.args, []string{"fetch", "--all", "--tags", "--quiet"}) || !got.remote {
		t.Errorf("fetch request: %+v", got)
	}
	fail := &mockGitWrapper{result: &runcmdStubResult{success: false, err: errors.New("exit status 128"), stderr: "fatal: unable to access"}}
	if err := (&GitCommands{ui: ui, executor: fail}).FetchTags(context.Background(), "/tmp/repo"); err == nil || !strings.Contains(err.Error(), "unable to access") {
		t.Errorf("expected git error, got %v", err)
	}
	if err := (&GitCommands{ui: ui, executor: &mockGitWrapper{}}).FetchTags(context.Background(), "--repo"); err == nil {
		t.Error("expected error for dirpath starting with '-'")
	}
}

func TestGitCommands_UnreachableTags(t *testing.T) {
	ui := clui.DefaultClui()
	mock := &mockGitWrapper{results: []runcmdResult{&runcmdStubResult{success: true, stdout: "v0.5.0\nv0.4.0\n"}}}
	tags, err := (&GitCommands{ui: ui, executor: mock}).UnreachableTags(context.Background(), "/tmp/repo", "v*", "v0.3.0")
	if err != nil || !reflect.DeepEqual(tags, []string{"v0.5.0", "v0.4.0"}) {
		t.Errorf("got %v (err %v)", tags, err)
	}
	want := []string{"tag", "--list", "--no-merged=HEAD", "--sort=-creatordate", "--contains=refs/tags/v0.3.0", "--", "v*"}
	if got := mock.requests[0].args; !reflect.DeepEqual(got, want) {
		t.Errorf("args: %v", got)
	}
	mock = &mockGitWrapper{}
	if tags, err := (&GitCommands{ui: ui, executor: mock}).UnreachableTags(context.Background(), "/tmp/repo", "", ""); err != nil || tags != nil {
		t.Errorf("got %v (err %v), want none", tags, err)
	}
	if got := mock.requests[0].args; !reflect.DeepEqual(got, want[:4]) {
		t.Errorf("args without base/match: %v", got)
	}
	fail := &mockGitWrapper{result: &runcmdStubResult{success: false, err: errors.New("exit status 128"), stderr: "fatal: not a git repository"}}
	if _, err := (&GitCommands{ui: ui, executor: fail}).UnreachableTags(context.Background(), "/tmp/repo", "", ""); err == nil {
		t.Error("expected git error")
	}
	if _, err := (&GitCommands{ui: ui, executor: &mockGitWrapper{}}).UnreachableTags(context.Background(), "--repo", "", ""); err == nil {
		t.Error("expected error for dirpath starting with '-'")
	}
}
