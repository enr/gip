package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/enr/gip/lib/core"
)

// Per-project "branches": extra local branches reported by status and
// fast-forwarded by pull. Projects without the field never reach this code
// beyond the early return, so their behaviour is unchanged.

// branchTarget returns the project path and the extra branches to handle, or
// ok=false when there is nothing to do (no branches configured, missing dir,
// unreadable current branch, ...).
func branchTarget(c *cli.Context, git *core.GitCommands, project gipProject) (line string, extras []string, ok bool) {
	if len(project.Branches) == 0 {
		return "", nil, false
	}
	line, err := project.resolvedPath()
	if err != nil || !isProjectDir(line) {
		return "", nil, false
	}
	ctx, cancel := opContext(c)
	defer cancel()
	current, err := git.CurrentBranch(ctx, line)
	if err != nil {
		return "", nil, false
	}
	extras = project.extraBranches(current)
	return line, extras, len(extras) > 0
}

func branchLabel(project gipProject, branch string) string {
	return fmt.Sprintf("%s@%s", project.Name, branch)
}

// statusExtraBranches reports ahead/behind of the configured non-current
// branches. It reads local refs only (run "gip fetch" to refresh upstreams).
func statusExtraBranches(c *cli.Context, git *core.GitCommands, t *tracker, project gipProject, f gitStateFilter) {
	if noopMode || f.dirty {
		return
	}
	line, extras, ok := branchTarget(c, git, project)
	if !ok {
		return
	}
	ctx, cancel := opContext(c)
	defer cancel()
	infos, err := git.BranchInfos(ctx, line, extras)
	if err != nil {
		t.expect(1)
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
		return
	}
	for _, info := range infos {
		t.expect(1)
		res := opResult{project: branchLabel(project, info.Name), localPath: line, branch: info.Name}
		sync := core.BranchSyncStatus{Branch: info.Name, Ahead: info.Ahead, Behind: info.Behind, NoRemote: info.NoRemote}
		skip := f.syncSkipReason(info.Ahead, info.Behind)
		switch {
		case !info.Exists:
			res.status, res.reason = opSkipped, "branch not found locally"
		case !info.NoRemote && info.Ahead == 0 && info.Behind == 0:
			res.status, res.reason = opSkipped, "in sync"
		case skip != "":
			res.status, res.reason = opSkipped, skip
		default:
			res.status = opOK
			res.state = &repoState{sync: sync}
			name := info.Name
			t.withOutput(func() {
				ui.Lifecyclef("%s [%s]  %s", line, name, t.colorSync(sync, false))
			})
		}
		t.record(res)
	}
}

// pullExtraBranches fast-forwards the configured non-current branches from
// their upstream. Non fast-forward updates are refused by git and reported as
// errors; local commits are never rewritten.
func pullExtraBranches(c *cli.Context, git *core.GitCommands, t *tracker, project gipProject, f gitStateFilter) {
	if project.pullNever() {
		return
	}
	line, extras, ok := branchTarget(c, git, project)
	if !ok {
		return
	}
	if noopMode {
		for _, b := range extras {
			t.expect(1)
			t.printNoop("%s → git fetch <remote> %s:%s  (in %s)", branchLabel(project, b), b, b, line)
			t.record(opResult{project: branchLabel(project, b), localPath: line, status: opOK, branch: b})
		}
		return
	}
	ctx, cancel := opContext(c)
	defer cancel()
	// --dirty refers to the worktree; --behind/--ahead are checked per branch
	// below (the checked-out branch may just have been pulled).
	// On error pullOne has already reported the same failed check: leave the
	// branches untouched.
	if skip, err := (gitStateFilter{dirty: f.dirty}).check(ctx, git, line); skip != "" || err != nil {
		return
	}
	infos, err := git.BranchInfos(ctx, line, extras)
	if err != nil {
		t.expect(1)
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
		return
	}
	for _, info := range infos {
		t.expect(1)
		t.record(pullExtraBranch(ctx, git, t, project, line, info, f))
	}
}

// pullExtraBranch fast-forwards a single extra branch, or reports why it was
// skipped.
func pullExtraBranch(ctx context.Context, git *core.GitCommands, t *tracker, project gipProject, line string, info core.BranchInfo, f gitStateFilter) opResult {
	res := opResult{project: branchLabel(project, info.Name), localPath: line, branch: info.Name}
	skip := f.syncSkipReason(info.Ahead, info.Behind)
	switch {
	case !info.Exists:
		res.status, res.reason = opSkipped, "branch not found locally"
	case info.NoRemote || info.Remote == "":
		res.status, res.reason = opSkipped, "no upstream configured"
	case skip != "":
		res.status, res.reason = opSkipped, skip
	case info.Ahead > 0 && info.Behind == 0:
		res.status, res.reason = opSkipped, "ahead of upstream (nothing to pull)"
	default:
		if err := git.FastForwardBranch(ctx, line, info.Remote, info.RemoteRef, info.Name); err != nil {
			res.status, res.err = opError, err
		} else {
			res.status = opOK
			t.withOutput(func() { ui.Lifecyclef("%s [%s]  up to date with %s", line, info.Name, info.Remote) })
		}
	}
	return res
}
