package main

import (
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
	line, err := projectPath(project.LocalPath)
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

// branchFilteredOut applies the --behind/--ahead filters to a single extra
// branch, using its own tracking state (not the checked-out branch's).
func branchFilteredOut(info core.BranchInfo, filterBehind, filterAhead bool) bool {
	if !filterBehind && !filterAhead {
		return false
	}
	return !(filterBehind && info.Behind > 0) && !(filterAhead && info.Ahead > 0)
}

// statusExtraBranches reports ahead/behind of the configured non-current
// branches. It reads local refs only (run "gip fetch" to refresh upstreams).
func statusExtraBranches(c *cli.Context, git *core.GitCommands, t *tracker, project gipProject, filterDirty, filterBehind, filterAhead bool) {
	if noopMode || filterDirty {
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
		switch {
		case !info.Exists:
			res.status, res.reason = opSkipped, "branch not found locally"
		case !info.NoRemote && info.Ahead == 0 && info.Behind == 0:
			res.status, res.reason = opSkipped, "in sync"
		case branchFilteredOut(info, filterBehind, filterAhead):
			res.status, res.reason = opSkipped, "not ahead or behind"
		default:
			res.status = opOK
			res.syncSet, res.syncAhead, res.syncBehind, res.syncNoRemote = true, info.Ahead, info.Behind, info.NoRemote
			info := info
			t.withOutput(func() {
				ui.Lifecyclef("%s [%s]  %s", line, info.Name, t.colorSync(info.Ahead, info.Behind, info.NoRemote, false))
			})
		}
		t.record(res)
	}
}

// pullExtraBranches fast-forwards the configured non-current branches from
// their upstream. Non fast-forward updates are refused by git and reported as
// errors; local commits are never rewritten.
func pullExtraBranches(c *cli.Context, git *core.GitCommands, t *tracker, project gipProject, filterDirty, filterBehind, filterAhead bool) {
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
	if skip, _, _ := filterByGitState(ctx, git, line, filterDirty, false, false); skip {
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
		res := opResult{project: branchLabel(project, info.Name), localPath: line, branch: info.Name}
		switch {
		case !info.Exists:
			res.status, res.reason = opSkipped, "branch not found locally"
		case info.NoRemote || info.Remote == "":
			res.status, res.reason = opSkipped, "no upstream configured"
		case branchFilteredOut(info, filterBehind, filterAhead):
			res.status, res.reason = opSkipped, "not ahead or behind"
		case info.Ahead > 0 && info.Behind == 0:
			res.status, res.reason = opSkipped, "ahead of upstream (nothing to pull)"
		default:
			if err := git.FastForwardBranch(ctx, line, info.Remote, info.RemoteRef, info.Name); err != nil {
				res.status, res.err = opError, err
			} else {
				res.status = opOK
				info := info
				t.withOutput(func() { ui.Lifecyclef("%s [%s]  up to date with %s", line, info.Name, info.Remote) })
			}
		}
		t.record(res)
	}
}
