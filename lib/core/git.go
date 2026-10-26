package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/enr/clui"
	"github.com/enr/go-commons/environment"
)

// gitResultError builds an error that includes git's stderr output (which is
// where git writes its "fatal: ..." diagnostic messages).
func gitResultError(result runcmdResult) error {
	msg := strings.TrimSpace(result.Stderr().String())
	if msg == "" {
		return result.Error()
	}
	return fmt.Errorf("%w: %s", result.Error(), msg)
}

// failureDetail is the inline detail of a failed git run: exit status and
// stderr, or the error itself when git did not exit on its own (killed by a
// timeout, not started), where the status is a meaningless -1.
func failureDetail(result runcmdResult) string {
	if result.ExitStatus() < 0 && result.Error() != nil {
		return result.Error().Error()
	}
	return fmt.Sprintf("(%d) %s", result.ExitStatus(), strings.TrimSpace(result.Stderr().String()))
}

// GitOption configures a GitCommands instance.
type GitOption func(*GitCommands)

// WithSharedOutput makes all git display sections share mu and call beforeDisplay
// (while holding mu) before writing to the UI. This lets a caller serialise
// progress-bar updates with git output without holding the lock across subprocesses.
func WithSharedOutput(mu *sync.Mutex, beforeDisplay func()) GitOption {
	return func(g *GitCommands) {
		g.outMu = mu
		g.beforeDisplay = beforeDisplay
	}
}

// NewGit is the factory function for GitCommands
func NewGit(ui *clui.Clui, opts ...GitOption) (*GitCommands, error) {
	executor, err := newGitExecutor(ui)
	if err != nil {
		return nil, err
	}
	g := &GitCommands{
		ui:            ui,
		executor:      executor,
		outMu:         &sync.Mutex{},
		beforeDisplay: func() {},
	}
	for _, opt := range opts {
		opt(g)
	}
	return g, nil
}

// ensureOut guarantees outMu and beforeDisplay are initialised even when
// GitCommands is constructed directly (e.g., in tests via struct literals).
func (g *GitCommands) ensureOut() {
	if g.outMu == nil {
		g.outMu = &sync.Mutex{}
	}
	if g.beforeDisplay == nil {
		g.beforeDisplay = func() {}
	}
}

// GitCommands ...
type GitCommands struct {
	ui            *clui.Clui
	executor      runcmdWrapper
	outMu         *sync.Mutex
	beforeDisplay func()

	showStashOnce sync.Once
	showStash     bool // "status --porcelain=v2 --show-stash" reports "# stash N" (git >= 2.35)
}

// Clone executes `git clone`
func (g *GitCommands) Clone(ctx context.Context, repourl string, dirpath string) error {
	g.ensureOut()
	var err error
	if strings.HasPrefix(repourl, "-") {
		return fmt.Errorf("invalid repourl: cannot start with '-'")
	}
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	g.ui.Confidentialf("Cloning %s to %s", repourl, dirpath)
	args := []string{
		"clone",
		"--",
		repourl,
		dirpath,
	}
	r := runcmdWrapperRequest{
		ctx:    ctx,
		args:   args,
		remote: true,
	}
	result := g.executor.exec(r)
	if !result.Success() {
		err = gitResultError(result)
	}
	g.outMu.Lock()
	g.beforeDisplay()
	g.ui.Title(dirpath)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("%s", failureDetail(result))
	}
	g.ui.Lifecycle(result.Stdout().String())
	g.outMu.Unlock()
	return err
}

// Pull executes `git pull`
func (g *GitCommands) Pull(ctx context.Context, dirpath string) error {
	g.ensureOut()
	var err error
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	g.ui.Confidentialf("Pulling %s", dirpath)
	args := []string{
		"pull",
	}
	r := runcmdWrapperRequest{
		ctx:        ctx,
		args:       args,
		workingDir: dirpath,
		remote:     true,
	}
	result := g.executor.exec(r)
	if !result.Success() {
		err = gitResultError(result)
	}
	g.outMu.Lock()
	g.beforeDisplay()
	g.ui.Title(dirpath)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("%s", failureDetail(result))
	}
	g.ui.Lifecycle(result.Stdout().String())
	g.outMu.Unlock()
	return err
}

// Status executes `git status`
func (g *GitCommands) Status(ctx context.Context, dirpath string, untracked bool) error {
	g.ensureOut()
	var err error
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	g.ui.Confidentialf("Status on %s", dirpath)
	r := runcmdWrapperRequest{
		ctx:        ctx,
		args:       statusArguments(untracked),
		workingDir: dirpath,
	}
	result := g.executor.exec(r)
	if !result.Success() {
		err = gitResultError(result)
	}
	gitOutput := result.Stdout().String()
	if len(gitOutput) == 0 && result.Success() {
		g.ui.Confidentialf("%s unmodified", dirpath)
	} else {
		g.outMu.Lock()
		g.beforeDisplay()
		g.ui.Title(dirpath)
		if !result.Success() {
			g.ui.Errorf("Error executing Git in %s", dirpath)
			g.ui.Errorf("%s", failureDetail(result))
		}
		g.ui.Lifecycle(gitOutput)
		g.outMu.Unlock()
	}
	return err
}

// Fetch executes `git fetch --all --prune`
func (g *GitCommands) Fetch(ctx context.Context, dirpath string) error {
	g.ensureOut()
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	g.ui.Confidentialf("Fetching %s", dirpath)
	r := runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"fetch", "--all", "--prune"},
		workingDir: dirpath,
		remote:     true,
	}
	result := g.executor.exec(r)
	var err error
	if !result.Success() {
		err = gitResultError(result)
	}
	g.outMu.Lock()
	g.beforeDisplay()
	g.ui.Title(dirpath)
	if !result.Success() {
		g.ui.Errorf("Error executing Git in %s", dirpath)
		g.ui.Errorf("%s", failureDetail(result))
	}
	g.ui.Lifecycle(result.Stdout().String())
	g.outMu.Unlock()
	return err
}

// CurrentBranch returns the name of the current branch, or "(detached)" for a detached HEAD.
func (g *GitCommands) CurrentBranch(ctx context.Context, dirpath string) (string, error) {
	if strings.HasPrefix(dirpath, "-") {
		return "", fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	r := runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"rev-parse", "--abbrev-ref", "HEAD"},
		workingDir: dirpath,
	}
	result := g.executor.exec(r)
	if !result.Success() {
		return "", gitResultError(result)
	}
	branch := strings.TrimSpace(result.Stdout().String())
	if branch == "HEAD" {
		branch = "(detached)"
	}
	return branch, nil
}

// BranchSyncStatus holds the upstream delta for the checked-out branch.
type BranchSyncStatus struct {
	// Branch is the checked-out branch, "(detached)" for a detached HEAD.
	Branch   string
	Ahead    int
	Behind   int
	NoRemote bool // true when no upstream tracking branch is configured
}

// SyncLabel returns a compact display string (e.g. "synced", "↑2", "↓3", "↑1↓2", "no-remote").
func (s BranchSyncStatus) SyncLabel() string {
	if s.NoRemote {
		return "no-remote"
	}
	if s.Ahead == 0 && s.Behind == 0 {
		return "synced"
	}
	if s.Ahead > 0 && s.Behind > 0 {
		return fmt.Sprintf("↑%d↓%d", s.Ahead, s.Behind)
	}
	if s.Ahead > 0 {
		return fmt.Sprintf("↑%d", s.Ahead)
	}
	return fmt.Sprintf("↓%d", s.Behind)
}

// DirtyStatus captures working-tree and index state.
type DirtyStatus struct {
	Staged    bool
	Unstaged  bool
	Untracked bool
	Stashed   bool
}

// Symbols returns compact status indicators (+*?$), empty when clean.
func (d DirtyStatus) Symbols() string {
	var b strings.Builder
	if d.Staged {
		b.WriteByte('+')
	}
	if d.Unstaged {
		b.WriteByte('*')
	}
	if d.Untracked {
		b.WriteByte('?')
	}
	if d.Stashed {
		b.WriteByte('$')
	}
	return b.String()
}

// IsDirty reports whether the working tree or index has any uncommitted change.
func (d DirtyStatus) IsDirty() bool {
	return d.Staged || d.Unstaged || d.Untracked
}

func parseStatusLine(line string, sync *BranchSyncStatus, dirty *DirtyStatus) {
	switch {
	case strings.HasPrefix(line, "# "):
		parseStatusHeader(line, sync, dirty)
	case strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 "):
		if len(line) >= 4 {
			if line[2] != '.' {
				dirty.Staged = true
			}
			if line[3] != '.' {
				dirty.Unstaged = true
			}
		}
	case strings.HasPrefix(line, "u "):
		dirty.Staged = true
	case strings.HasPrefix(line, "? "):
		dirty.Untracked = true
	}
}

// parseStatusHeader parses one "# ..." header line of porcelain v2 status.
func parseStatusHeader(line string, sync *BranchSyncStatus, dirty *DirtyStatus) {
	switch {
	case strings.HasPrefix(line, "# branch.head "):
		sync.Branch = strings.TrimPrefix(line, "# branch.head ")
	case strings.HasPrefix(line, "# branch.ab "):
		sync.NoRemote = false
		fmt.Sscanf(strings.TrimPrefix(line, "# branch.ab "), "+%d -%d", &sync.Ahead, &sync.Behind)
	case strings.HasPrefix(line, "# stash "):
		dirty.Stashed = true
	}
}

// statusConfig makes "git status" decide whether a file changed by its size
// and mtime only. The full stat check also compares inode, device, owner and
// ctime, which never match in an index written by another git (Git for
// Windows on a repository that WSL reads through /mnt/c): every file is then
// re-read and hashed on every status, which on a large repository takes tens
// of seconds.
var statusConfig = []string{"-c", "core.checkStat=minimal", "-c", "core.trustctime=false"}

// StatusInfo runs "git status --porcelain=v2 --branch" to collect upstream
// delta and working-tree indicators, stash included: in the same call when
// git reports it there, otherwise with a "git stash list".
func (g *GitCommands) StatusInfo(ctx context.Context, dirpath string) (BranchSyncStatus, DirtyStatus, error) {
	if strings.HasPrefix(dirpath, "-") {
		return BranchSyncStatus{}, DirtyStatus{}, fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	showStash := g.statusShowsStash(ctx)
	args := append(append([]string{}, statusConfig...), "status", "--porcelain=v2", "--branch")
	if showStash {
		args = append(args, "--show-stash")
	}
	r := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       args,
		workingDir: dirpath,
	})
	if !r.Success() {
		return BranchSyncStatus{}, DirtyStatus{}, gitResultError(r)
	}

	var sync BranchSyncStatus
	var dirty DirtyStatus
	sync.NoRemote = true

	for _, line := range strings.Split(r.Stdout().String(), "\n") {
		parseStatusLine(line, &sync, &dirty)
	}
	if showStash {
		return sync, dirty, nil
	}

	stash := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"stash", "list"},
		workingDir: dirpath,
	})
	if stash.Success() && strings.TrimSpace(stash.Stdout().String()) != "" {
		dirty.Stashed = true
	}

	return sync, dirty, nil
}

// statusShowsStash reports whether the installed git prints the stash count in
// porcelain v2 status (git >= 2.35), asking "git version" once.
func (g *GitCommands) statusShowsStash(ctx context.Context) bool {
	g.showStashOnce.Do(func() {
		r := g.executor.exec(runcmdWrapperRequest{ctx: ctx, args: []string{"version"}})
		g.showStash = r.Success() && gitVersionAtLeast(r.Stdout().String(), 2, 35)
	})
	return g.showStash
}

// gitVersionAtLeast parses "git version X.Y..." output (also "2.41.0.windows.1"
// or "2.39.3 (Apple Git-146)"); unparsable output counts as older.
func gitVersionAtLeast(output string, major, minor int) bool {
	var gotMajor, gotMinor int
	if _, err := fmt.Sscanf(strings.TrimSpace(output), "git version %d.%d", &gotMajor, &gotMinor); err != nil {
		return false
	}
	return gotMajor > major || (gotMajor == major && gotMinor >= minor)
}

// LastCommit returns the subject line and relative date of the most recent commit.
func (g *GitCommands) LastCommit(ctx context.Context, dirpath string) (subject, relDate string, err error) {
	if strings.HasPrefix(dirpath, "-") {
		return "", "", fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	r := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"log", "-1", "--format=%s%n%cr"},
		workingDir: dirpath,
	})
	if !r.Success() {
		return "", "", gitResultError(r)
	}
	out := strings.TrimSpace(r.Stdout().String())
	if out == "" {
		return "(no commits)", "", nil
	}
	parts := strings.SplitN(out, "\n", 2)
	subject = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		relDate = strings.TrimSpace(parts[1])
	}
	return subject, relDate, nil
}

func statusArguments(untracked bool) []string {
	untrackedFlag := "=no"
	if untracked {
		untrackedFlag = ""
	}
	args := append(append([]string{}, statusConfig...),
		"status",
		"--porcelain",
		fmt.Sprintf("--untracked-files%s", untrackedFlag),
	)
	return args
}

func gitExecutablePath() (string, error) {
	gitExecutable := environment.Which("git")
	if gitExecutable == "" {
		return "", fmt.Errorf(`git executable not found`)
	}
	return gitExecutable, nil
}

// BranchInfo describes a local branch and its relation to its upstream.
type BranchInfo struct {
	Name       string
	Exists     bool   // false when the branch is not present locally
	Remote     string // remote name of the upstream, empty when none
	RemoteRef  string // upstream ref on the remote (e.g. refs/heads/main), empty when none
	Ahead      int
	Behind     int
	NoRemote   bool // no upstream configured, or upstream is gone
	CheckedOut bool // the branch is HEAD of this worktree
}

func validRefArg(s string) bool {
	return s != "" && !strings.HasPrefix(s, "-")
}

func parseTrack(track string) (ahead, behind int, gone bool) {
	if strings.TrimSpace(track) == "gone" {
		return 0, 0, true
	}
	for _, part := range strings.Split(track, ",") {
		part = strings.TrimSpace(part)
		fmt.Sscanf(part, "ahead %d", &ahead)
		fmt.Sscanf(part, "behind %d", &behind)
	}
	return ahead, behind, false
}

const branchInfoFormat = "--format=%(refname:short)%09%(upstream:short)%09%(upstream:remotename)%09%(upstream:track,nobracket)%09%(HEAD)%09%(upstream:remoteref)"

// parseBranchInfos parses the output of for-each-ref with branchInfoFormat.
func parseBranchInfos(stdout string) map[string]BranchInfo {
	found := make(map[string]BranchInfo)
	for _, line := range strings.Split(stdout, "\n") {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) < 6 {
			continue
		}
		info := BranchInfo{Name: f[0], Exists: true, Remote: f[2], RemoteRef: f[5], CheckedOut: f[4] == "*"}
		var gone bool
		info.Ahead, info.Behind, gone = parseTrack(f[3])
		info.NoRemote = f[1] == "" || gone
		found[info.Name] = info
	}
	return found
}

// BranchInfos returns one entry per requested branch (same order), reading
// local refs only: no network access, no change to the repository.
func (g *GitCommands) BranchInfos(ctx context.Context, dirpath string, branches []string) ([]BranchInfo, error) {
	if strings.HasPrefix(dirpath, "-") {
		return nil, fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	args := []string{"for-each-ref", branchInfoFormat}
	for _, b := range branches {
		if !validRefArg(b) {
			return nil, fmt.Errorf("invalid branch name %q", b)
		}
		args = append(args, "refs/heads/"+b)
	}
	if len(branches) == 0 {
		return nil, nil
	}
	r := g.executor.exec(runcmdWrapperRequest{ctx: ctx, args: args, workingDir: dirpath})
	if !r.Success() {
		return nil, gitResultError(r)
	}
	found := parseBranchInfos(r.Stdout().String())
	out := make([]BranchInfo, 0, len(branches))
	for _, b := range branches {
		if info, ok := found[b]; ok {
			out = append(out, info)
		} else {
			out = append(out, BranchInfo{Name: b})
		}
	}
	return out, nil
}

// FastForwardBranch updates the local branch from its upstream with
// "git fetch <remote> <remoteRef>:refs/heads/<branch>". remoteRef is the
// upstream ref on the remote (BranchInfo.RemoteRef); when empty the same name
// as the local branch is assumed. Git itself refuses non fast-forward updates
// and branches checked out in a worktree, so the operation never rewrites
// local commits.
func (g *GitCommands) FastForwardBranch(ctx context.Context, dirpath, remote, remoteRef, branch string) error {
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	if !validRefArg(remote) || !validRefArg(branch) || strings.HasPrefix(remoteRef, "-") {
		return fmt.Errorf("invalid remote %q, upstream %q or branch %q", remote, remoteRef, branch)
	}
	ref := "refs/heads/" + branch
	if remoteRef == "" {
		remoteRef = ref
	}
	r := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"fetch", remote, remoteRef + ":" + ref},
		workingDir: dirpath,
		remote:     true,
	})
	if !r.Success() {
		return gitResultError(r)
	}
	return nil
}

// TagInfo describes the latest tag reachable from HEAD and how far HEAD has
// moved since then.
type TagInfo struct {
	// Tag is the latest reachable tag, empty when no tag is reachable from HEAD.
	Tag string
	// CommitsSince counts the commits between Tag and HEAD; when Tag is empty
	// it is the total number of commits reachable from HEAD.
	CommitsSince int
	// TagDate is the relative creation date of Tag (tagger date for annotated
	// tags, commit date for lightweight ones).
	TagDate string
	// NoCommits is true for a repository without commits (unborn HEAD).
	NoCommits bool
}

// Unreleased reports whether HEAD carries commits not covered by a tag: either
// commits after the latest tag, or commits in a repository never tagged.
func (t TagInfo) Unreleased() bool {
	return !t.NoCommits && (t.Tag == "" || t.CommitsSince > 0)
}

// parseDescribeLong splits "git describe --long" output ("<tag>-<n>-g<sha>")
// into tag and commit count. Tags may contain dashes, so it splits from the right.
func parseDescribeLong(out string) (tag string, count int, err error) {
	out = strings.TrimSpace(out)
	shaSep := strings.LastIndex(out, "-")
	if shaSep <= 0 {
		return "", 0, fmt.Errorf("unexpected git describe output %q", out)
	}
	countSep := strings.LastIndex(out[:shaSep], "-")
	if countSep <= 0 {
		return "", 0, fmt.Errorf("unexpected git describe output %q", out)
	}
	if _, err := fmt.Sscanf(out[countSep+1:shaSep], "%d", &count); err != nil {
		return "", 0, fmt.Errorf("unexpected git describe output %q", out)
	}
	return out[:countSep], count, nil
}

// isNoTagError reports whether git describe failed only because no tag is
// reachable (as opposed to a broken repository).
func isNoTagError(stderr string) bool {
	return strings.Contains(stderr, "No names found") || strings.Contains(stderr, "No tags can describe")
}

// FetchTags downloads the tags of every remote (git fetch --all --tags) without
// printing anything, so that LatestTag also sees tags pushed elsewhere. Local
// tags missing on the remotes are kept (no --prune).
func (g *GitCommands) FetchTags(ctx context.Context, dirpath string) error {
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	r := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"fetch", "--all", "--tags", "--quiet"},
		workingDir: dirpath,
		remote:     true,
	})
	if !r.Success() {
		return gitResultError(r)
	}
	return nil
}

// LatestTag returns the latest tag reachable from HEAD and the number of
// commits made after it. match, when not empty, is a glob restricting the
// tags considered (git describe --match). It reads local refs only.
func (g *GitCommands) LatestTag(ctx context.Context, dirpath, match string) (TagInfo, error) {
	if strings.HasPrefix(dirpath, "-") {
		return TagInfo{}, fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	args := []string{"describe", "--tags", "--long"}
	if match != "" {
		args = append(args, "--match="+match)
	}
	r := g.executor.exec(runcmdWrapperRequest{ctx: ctx, args: args, workingDir: dirpath})
	if r.Success() {
		tag, count, err := parseDescribeLong(r.Stdout().String())
		if err != nil {
			return TagInfo{}, err
		}
		info := TagInfo{Tag: tag, CommitsSince: count}
		date := g.executor.exec(runcmdWrapperRequest{
			ctx:        ctx,
			args:       []string{"for-each-ref", "--format=%(creatordate:relative)", "refs/tags/" + tag},
			workingDir: dirpath,
		})
		if date.Success() {
			info.TagDate = strings.TrimSpace(date.Stdout().String())
		}
		return info, nil
	}
	if !isNoTagError(r.Stderr().String()) {
		return TagInfo{}, gitResultError(r)
	}
	// No reachable tag: count every commit, or flag an empty repository.
	total := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"rev-list", "--count", "HEAD", "--"},
		workingDir: dirpath,
	})
	if !total.Success() {
		head := g.executor.exec(runcmdWrapperRequest{
			ctx:        ctx,
			args:       []string{"rev-parse", "--verify", "-q", "HEAD"},
			workingDir: dirpath,
		})
		if !head.Success() {
			return TagInfo{NoCommits: true}, nil
		}
		return TagInfo{}, gitResultError(total)
	}
	var info TagInfo
	if _, err := fmt.Sscanf(strings.TrimSpace(total.Stdout().String()), "%d", &info.CommitsSince); err != nil {
		return TagInfo{}, fmt.Errorf("unexpected git rev-list output %q", total.Stdout().String())
	}
	return info, nil
}

// UnreachableTags returns the tags not reachable from HEAD that descend from
// base (every unreachable tag when base is empty), most recent first. They
// reveal a release tagged on a commit later rewritten or never merged into
// the checked-out branch, which LatestTag cannot see. match, when not empty,
// is a glob restricting the tags considered. It reads local refs only.
func (g *GitCommands) UnreachableTags(ctx context.Context, dirpath, match, base string) ([]string, error) {
	if strings.HasPrefix(dirpath, "-") {
		return nil, fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	args := []string{"tag", "--list", "--no-merged=HEAD", "--sort=-creatordate"}
	if base != "" {
		args = append(args, "--contains=refs/tags/"+base)
	}
	if match != "" {
		args = append(args, "--", match)
	}
	r := g.executor.exec(runcmdWrapperRequest{ctx: ctx, args: args, workingDir: dirpath})
	if !r.Success() {
		return nil, gitResultError(r)
	}
	var tags []string
	for _, line := range strings.Split(r.Stdout().String(), "\n") {
		if tag := strings.TrimSpace(line); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// RemoteURLs returns the configured URL of each remote, keyed by remote name.
// A repository without remotes yields an empty map. It reads the local
// configuration only.
func (g *GitCommands) RemoteURLs(ctx context.Context, dirpath string) (map[string]string, error) {
	if strings.HasPrefix(dirpath, "-") {
		return nil, fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	r := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"config", "--get-regexp", `^remote\..*\.url$`},
		workingDir: dirpath,
	})
	urls := make(map[string]string)
	if !r.Success() {
		// Exit status 1: no key matched, i.e. no remote configured.
		if r.ExitStatus() == 1 && strings.TrimSpace(r.Stderr().String()) == "" {
			return urls, nil
		}
		return nil, gitResultError(r)
	}
	for _, line := range strings.Split(r.Stdout().String(), "\n") {
		key, value, ok := strings.Cut(strings.TrimRight(line, "\r"), " ")
		if !ok {
			continue
		}
		// remote.<name>.url: the name itself may contain dots.
		name := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")
		if _, seen := urls[name]; !seen {
			urls[name] = strings.TrimSpace(value)
		}
	}
	return urls, nil
}

// LsRemote checks that repourl is reachable and readable with
// "git ls-remote <repourl> HEAD". dirpath, when not empty, is the working
// directory (its core.sshCommand is honoured). Nothing is changed locally.
func (g *GitCommands) LsRemote(ctx context.Context, dirpath, repourl string) error {
	if strings.HasPrefix(dirpath, "-") {
		return fmt.Errorf("invalid dirpath: cannot start with '-'")
	}
	if strings.HasPrefix(repourl, "-") {
		return fmt.Errorf("invalid repository URL: cannot start with '-'")
	}
	r := g.executor.exec(runcmdWrapperRequest{
		ctx:        ctx,
		args:       []string{"ls-remote", repourl, "HEAD"},
		workingDir: dirpath,
		remote:     true,
	})
	if !r.Success() {
		return gitResultError(r)
	}
	return nil
}
