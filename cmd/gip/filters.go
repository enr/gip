package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/enr/gip/lib/core"
)

// gitStateFilter is the --dirty/--behind/--ahead/--unsynced selection. Every
// command decides through it whether a repository matches.
type gitStateFilter struct {
	dirty, behind, ahead bool
}

func newGitStateFilter(c *cli.Context) gitStateFilter {
	unsynced := c.Bool("unsynced")
	return gitStateFilter{
		dirty:  c.Bool("dirty"),
		behind: c.Bool("behind") || unsynced,
		ahead:  c.Bool("ahead") || unsynced,
	}
}

func (f gitStateFilter) active() bool {
	return f.dirty || f.behind || f.ahead
}

// syncSkipReason applies --behind/--ahead: matching either one is enough.
// It returns "" when the counts match.
func (f gitStateFilter) syncSkipReason(ahead, behind int) string {
	if !f.behind && !f.ahead {
		return ""
	}
	if (f.behind && behind > 0) || (f.ahead && ahead > 0) {
		return ""
	}
	switch {
	case f.behind && f.ahead:
		return "not ahead or behind"
	case f.behind:
		return "not behind"
	default:
		return "not ahead"
	}
}

// skipReason returns why a repository in this state is excluded, "" when it matches.
func (f gitStateFilter) skipReason(st repoState) string {
	if f.dirty && !st.dirty.IsDirty() {
		return "not dirty"
	}
	return f.syncSkipReason(st.sync.Ahead, st.sync.Behind)
}

// check reads the repository state and applies the filter. An unreadable
// state is an error, never a match.
func (f gitStateFilter) check(ctx context.Context, git *core.GitCommands, dirpath string) (skipReason string, err error) {
	if !f.active() {
		return "", nil
	}
	st, err := readRepoState(ctx, git, dirpath)
	if err != nil {
		return "", err
	}
	return f.skipReason(st), nil
}

// repoState is the upstream delta and working-tree state of a repository.
type repoState struct {
	sync  core.BranchSyncStatus
	dirty core.DirtyStatus
}

func readRepoState(ctx context.Context, git *core.GitCommands, dirpath string) (repoState, error) {
	sync, dirty, err := git.StatusInfo(ctx, dirpath)
	if err != nil {
		return repoState{}, fmt.Errorf("checking repository state: %w", err)
	}
	return repoState{sync: sync, dirty: dirty}, nil
}

// cell returns s, or "—" for an empty table cell.
func cell(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
