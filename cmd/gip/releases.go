package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/urfave/cli/v2"

	"github.com/enr/gip/lib/core"
)

// "releases": latest tag reachable from HEAD and commits made after it, to
// spot the projects that have unreleased work.

var releasesFlags = append(parallelFlags,
	&cli.BoolFlag{Name: "pending", Aliases: []string{"p"}, Usage: "only include projects with commits after the latest tag (or never tagged)"},
	&cli.BoolFlag{Name: "tagged", Usage: "only include projects with at least one tag (hide \"(none)\" and \"(no commits)\")"},
	&cli.IntFlag{Name: "min", Usage: "only include projects with at least `NUM` commits after the latest tag (total commits when never tagged, unless --tagged)"},
	&cli.StringFlag{Name: "match", Usage: "only consider tags matching this glob (git describe --match), e.g. 'v*'"},
	&cli.BoolFlag{Name: "fetch", Usage: "fetch tags from the remotes (git fetch --all --tags) before reading them"},
)

var commandReleases = cli.Command{
	Name:    "releases",
	Aliases: []string{"r", "rel"},
	Usage:   "show the latest tag and the commits made after it for each project",
	Description: `Prints, for every project, the latest tag reachable from the checked-out
branch (HEAD) and the number of commits made after it: projects with
commits after their latest tag are candidates for a new release.

Lightweight and annotated tags are both considered. Only local refs are
read: use --fetch (or run "gip fetch" first) to see tags pushed elsewhere.
A fetched tag is listed only once its commit is in the checked-out branch:
a tag on commits not pulled yet is not reachable from HEAD.

A warning is printed for tags that follow the latest reachable one but are
not in the checked-out branch (e.g. a release tag left on a commit rewritten
before being merged), since COMMITS is then counted from an older tag.

   gip releases
   gip releases --pending
   gip releases --tagged --pending
   gip releases --min 5
   gip releases --match 'v*'
   gip releases --fetch --pending`,
	Action:       doReleases,
	Flags:        releasesFlags,
	BashComplete: bashCompleteWithFlags(releasesFlags),
}

type releaseEntry struct {
	name   string
	path   string
	branch string
	info   core.TagInfo
	err    error
	note   string // "(missing)" or "(noop)" in place of tag info
}

type releasesOptions struct {
	match           string
	fetch           bool
	pending, tagged bool
	min             int // 0 = no minimum
	filter          gitStateFilter
}

// filtering reports whether --pending/--tagged/--min can hide projects.
func (o releasesOptions) filtering() bool {
	return o.pending || o.tagged || o.min > 0
}

// skipReason returns why a project with the given tag info is filtered out
// by --tagged/--pending/--min, or "" when it must be listed. --min compares
// the COMMITS value, i.e. the total commits of a project never tagged.
func (o releasesOptions) skipReason(info core.TagInfo) string {
	switch {
	case o.filtering() && info.NoCommits:
		return "no commits"
	case o.tagged && info.Tag == "":
		return "no tags"
	case o.pending && !info.Unreleased():
		return "no commits after latest tag"
	case info.CommitsSince < o.min:
		return fmt.Sprintf("fewer than %d commits after latest tag", o.min)
	}
	return ""
}

func doReleases(c *cli.Context) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, warnings, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, tagFilterValue(c))
	projects = filterDisabled(projects)
	t := newTracker(len(projects))
	git, err := core.NewGit(ui, core.WithSharedOutput(t.sharedOutput()))
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}
	jobs, err := resolveJobs(c)
	if err != nil {
		return exitErrorf(1, "%v", err)
	}
	if c.Int("min") < 0 {
		return exitErrorf(1, "--min must be >= 0, got %d", c.Int("min"))
	}
	opts := releasesOptions{
		match: c.String("match"), fetch: c.Bool("fetch"), pending: c.Bool("pending"), tagged: c.Bool("tagged"), min: c.Int("min"),
		filter: newGitStateFilter(c),
	}

	var entriesMu sync.Mutex
	entries := make([]releaseEntry, 0, len(projects))
	add := func(e releaseEntry) {
		entriesMu.Lock()
		entries = append(entries, e)
		entriesMu.Unlock()
	}
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for _, project := range projects {
		project := project
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if e, ok := releaseProject(c, git, t, project, opts); ok {
				add(e)
			}
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("releases", warnings)
	} else {
		printReleasesTable(entries, t)
		t.printSummary(c.Bool("errors-last"), c.Bool("show-skipped"))
	}
	return buildError(t.errors())
}

// releaseProject collects the tag info of one project and records its outcome.
// It returns ok=false when the project must not appear in the table.
func releaseProject(c *cli.Context, git *core.GitCommands, t *tracker, project gipProject, opts releasesOptions) (releaseEntry, bool) {
	entry := releaseEntry{name: project.Name}
	line, err := project.resolvedPath()
	if err != nil {
		entry.err = err
		t.record(opResult{project: project.Name, status: opError, err: err})
		return entry, true
	}
	entry.path = line
	if !isProjectDir(line) {
		entry.note = "(missing)"
		t.withOutput(func() { warnMissingDir(line) })
		if noopMode {
			t.printNoop("%s → SKIPPED  (directory missing)", project.Name)
		}
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: "local dir missing"})
		return entry, !opts.filtering() && !opts.filter.active()
	}
	if noopMode {
		if opts.fetch {
			t.printNoop("%s → git fetch --all --tags  (in %s)", project.Name, line)
		}
		t.printNoop("%s → git describe --tags --long  (in %s)", project.Name, line)
		entry.note = "(noop)"
		t.record(opResult{project: project.Name, localPath: line, status: opOK})
		return entry, true
	}
	ctx, cancel := opContext(c)
	defer cancel()
	fail := func(err error) (releaseEntry, bool) {
		entry.err = err
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
		return entry, true
	}
	if opts.fetch {
		if err := git.FetchTags(ctx, line); err != nil {
			return fail(fmt.Errorf("fetching tags: %w", err))
		}
	}
	reason, err := opts.filter.check(ctx, git, line)
	if err != nil {
		return fail(err)
	}
	if reason != "" {
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
		return entry, false
	}
	if entry.info, err = git.LatestTag(ctx, line, opts.match); err != nil {
		return fail(err)
	}
	var unreachable []string
	if !entry.info.NoCommits {
		if unreachable, err = git.UnreachableTags(ctx, line, opts.match, entry.info.Tag); err != nil {
			return fail(err)
		}
		warnUnreachableTags(t, project.Name, unreachable)
	}
	if reason := opts.skipReason(entry.info); reason != "" {
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
		return entry, false
	}
	entry.branch, _ = git.CurrentBranch(ctx, line)
	t.record(opResult{
		project: project.Name, localPath: line, status: opOK, branch: entry.branch,
		tagSet: !entry.info.NoCommits, lastTag: entry.info.Tag, commitsSinceTag: entry.info.CommitsSince,
		tagDate: entry.info.TagDate, unreleased: entry.info.Unreleased(), unreachableTags: unreachable,
	})
	return entry, true
}

// warnUnreachableTags reports tags newer than the latest reachable one but not
// in the checked-out branch, e.g. a release tag left on a commit rewritten
// before being merged: COMMITS is then counted from an older tag.
func warnUnreachableTags(t *tracker, name string, tags []string) {
	if len(tags) == 0 {
		return
	}
	t.withOutput(func() {
		ui.Warnf("%s: tag %s not reachable from HEAD (commits counted from the latest reachable tag)", name, strings.Join(tags, ", "))
	})
}

// releaseCells returns the TAG, COMMITS and DATE cells for one entry.
func releaseCells(e releaseEntry, t *tracker) (tag, commits, date string) {
	switch {
	case e.err != nil:
		return fmt.Sprintf("ERROR: %v", e.err), "—", "—"
	case e.note != "":
		return e.note, "—", "—"
	case e.info.NoCommits:
		return "(no commits)", "—", "—"
	}
	tag, date = e.info.Tag, e.info.TagDate
	if tag == "" {
		tag = t.applyStyle(styleWhite, "(none)")
		date = "—"
	}
	commits = strconv.Itoa(e.info.CommitsSince)
	if e.info.Unreleased() {
		commits = t.applyStyle(styleSkip, commits)
	} else {
		commits = t.applyStyle(styleOK, commits)
	}
	return tag, commits, date
}

func printReleasesTable(entries []releaseEntry, t *tracker) {
	t.clearProgress() // erase the progress bar before the table prints to stdout
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	table := [][]string{{"NAME", "TAG", "COMMITS", "TAG DATE", "BRANCH", "PATH"}}
	for _, e := range entries {
		tag, commits, date := releaseCells(e, t)
		branch := e.branch
		if branch == "" {
			branch = "—"
		}
		table = append(table, []string{e.name, tag, commits, date, branch, e.path})
	}
	renderTable(table, 2)
}
