package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/enr/gip/lib/core"

	"github.com/urfave/cli/v2"
	yaml "gopkg.in/yaml.v3"
)

var tagFlag = &cli.StringFlag{
	Name:  "tag",
	Usage: "filter projects by tag (comma-separated, OR logic): --tag work,js",
}

var errorsLastFlag = &cli.BoolFlag{
	Name:  "errors-last",
	Usage: "print a grouped error section after the summary",
}

var extendedFlag = &cli.BoolFlag{
	Name:  "extended",
	Usage: "show branch sync status, dirty indicators, and last commit info (requires extra git calls per repo)",
}

var dirtyFilterFlag = &cli.BoolFlag{
	Name:  "dirty",
	Usage: "only include repos with uncommitted changes (staged, unstaged, or untracked)",
}

var behindFilterFlag = &cli.BoolFlag{
	Name:  "behind",
	Usage: "only include repos behind their upstream (includes diverged); requires up-to-date remote refs — run gip fetch first",
}

const maxJobs = 32

var parallelFlags = []cli.Flag{
	&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}, Value: 4, Usage: fmt.Sprintf("maximum number of repos to operate on concurrently (1-%d)", maxJobs)},
	&cli.IntFlag{Name: "timeout", Aliases: []string{"t"}, Value: 0, Usage: "per-operation timeout in seconds (0 = no timeout)"},
	tagFlag,
	errorsLastFlag,
	dirtyFilterFlag,
	behindFilterFlag,
}

func resolveJobs(c *cli.Context) (int, error) {
	n := c.Int("jobs")
	if n < 1 {
		return 0, fmt.Errorf("--jobs must be at least 1, got %d", n)
	}
	if n > maxJobs {
		return 0, fmt.Errorf("--jobs must be at most %d, got %d", maxJobs, n)
	}
	return n, nil
}

var commands = []*cli.Command{
	&commandStatus,
	&commandStatusFull,
	&commandList,
	&commandPull,
	&commandFetch,
	&commandBranch,
	&commandExec,
	&commandInit,
}

var commandStatus = cli.Command{
	Name:        "status",
	Aliases:     []string{"s"},
	Usage:       "show modified files in projects",
	Description: `Prints modified files.`,
	Action:      doStatus,
	Flags:       parallelFlags,
}

var commandStatusFull = cli.Command{
	Name:        "statusfull",
	Aliases:     []string{"sf"},
	Usage:       "show modified and new files in projects",
	Description: `Prints modified files and new ones.`,
	Action:      doStatusFull,
	Flags:       parallelFlags,
}

var commandList = cli.Command{
	Name:        "list",
	Aliases:     []string{"ls"},
	Usage:       "list registered projects",
	Description: `List projects in a table with name, path, policy, provider and tags.`,
	Action:      doList,
	Flags: []cli.Flag{
		tagFlag,
		&cli.BoolFlag{Name: "all", Aliases: []string{"a"}, Usage: "include disabled projects in the list"},
		extendedFlag,
		dirtyFilterFlag,
		behindFilterFlag,
	},
}

var commandPull = cli.Command{
	Name:        "pull",
	Usage:       "update projects from remote repositories",
	Description: `Pull projects`,
	Action:      doPull,
	Flags: append(parallelFlags,
		&cli.BoolFlag{Name: "all", Aliases: []string{"a"}, Usage: `Pull all registered projects doing a checkout if needed. Otherwise only the projects already present are updated.`},
	),
}

var commandFetch = cli.Command{
	Name:        "fetch",
	Usage:       "fetch remote refs for all projects without merging",
	Description: `Executes "git fetch --all --prune" for each project. Projects with pull_policy "never" are skipped.`,
	Action:      doFetch,
	Flags:       parallelFlags,
}

var commandBranch = cli.Command{
	Name:        "branch",
	Aliases:     []string{"br"},
	Usage:       "show current branch for each project",
	Description: `Prints the current branch (or "(detached)" for a detached HEAD) for every project in a table.`,
	Action:      doBranch,
	Flags:       append(parallelFlags, extendedFlag),
}

var commandExec = cli.Command{
	Name:  "exec",
	Usage: "execute an arbitrary command in each project directory",
	Description: `Execute an arbitrary command in each project directory.
Use -- to separate gip flags from the command and its arguments:

   gip exec -- git fetch --prune
   gip exec -j 8 -- make test`,
	Action: doExec,
	Flags:  parallelFlags,
}

var commandInit = cli.Command{
	Name:        "init",
	Usage:       "scan a directory for git repositories and generate configuration",
	Description: `Scans a directory recursively for git repositories and writes a gip configuration file.`,
	Action:      doInit,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "output file path (default: ./.gip)"},
		&cli.BoolFlag{Name: "force", Usage: "overwrite existing config without prompting"},
		&cli.IntFlag{Name: "depth", Value: 5, Usage: "maximum directory scan depth"},
	},
}

func doStatus(c *cli.Context) error {
	return gitStatus(c, false)
}

func doStatusFull(c *cli.Context) error {
	return gitStatus(c, true)
}

// opContext builds a context for a single git operation, respecting --timeout.
func opContext(c *cli.Context) (context.Context, context.CancelFunc) {
	secs := c.Int("timeout")
	if secs > 0 {
		return context.WithTimeout(context.Background(), time.Duration(secs)*time.Second)
	}
	return context.Background(), func() {}
}

func gitStatus(c *cli.Context, untracked bool) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, warnings, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	projects = filterDisabled(projects)
	t := newTracker(len(projects))
	git, err := core.NewGit(ui, core.WithSharedOutput(t.sharedOutput()))
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}
	filterDirty := c.Bool("dirty")
	filterBehind := c.Bool("behind")

	jobs, err := resolveJobs(c)
	if err != nil {
		return exitErrorf(1, "%v", err)
	}

	untrackedFlag := "--untracked-files=no"
	if untracked {
		untrackedFlag = "--untracked-files"
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

			line, err := projectPath(project.LocalPath)
			if err != nil {
				t.record(opResult{project: project.Name, status: opError, err: err})
				return
			}
			if !isProjectDir(line) {
				t.withOutput(func() { warnMissingDir(line) })
				t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: "not a project dir"})
				return
			}
			if noopMode {
				t.printNoop("%s → git status --porcelain %s  (in %s)", project.Name, untrackedFlag, line)
				t.record(opResult{project: project.Name, localPath: line, status: opOK})
				return
			}
			ctx, cancel := opContext(c)
			defer cancel()
			if skip, reason, _ := filterByGitState(ctx, git, line, filterDirty, filterBehind); skip {
				t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
				return
			}
			if err = git.Status(ctx, line, untracked); err != nil {
				t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
			} else {
				t.record(opResult{project: project.Name, localPath: line, status: opOK})
			}
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("status", warnings)
	} else {
		t.printSummary(c.Bool("errors-last"))
	}
	return buildError(t.errors())
}

func buildError(errors map[string]error) error {
	if len(errors) == 0 {
		return nil
	}
	keys := make([]string, 0, len(errors))
	for k := range errors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buffer bytes.Buffer
	for _, key := range keys {
		buffer.WriteString(fmt.Sprintf("- %s: %v\n", key, errors[key]))
	}
	return exitError(1, buffer.String())
}

// listRow holds the display data for one project in the list table.
type listRow struct {
	name, path, policy, provider, tags string
	repository                         string
	missingDir                         bool
	pathErr                            error
	rawPath                            string // resolved path without display decorations
	// extended fields (populated when --extended/--dirty/--behind is set)
	branch       string
	syncAhead    int
	syncBehind   int
	syncNoRemote bool
	dirtySymbols string // "+*?$" or "" when clean/not collected
	hasExtended  bool
}

func buildListRows(projects []gipProject) ([]listRow, bool, map[string]error) {
	hasTags := false
	rows := make([]listRow, 0, len(projects))
	errs := map[string]error{}
	for _, project := range projects {
		policy := project.PullPolicy
		if policy == "" {
			policy = "default"
		}
		provider := project.repoProvider()
		if provider == "" {
			provider = "—"
		}
		tags := "—"
		if len(project.Tags) > 0 {
			hasTags = true
			tags = strings.Join(project.Tags, ", ")
		}
		displayName := project.Name
		if project.Disabled {
			displayName += " (disabled)"
		}
		localPath, err := projectPath(project.LocalPath)
		if err != nil {
			errs[project.Name] = err
			rows = append(rows, listRow{
				name: displayName, path: project.LocalPath + " (ERROR)",
				policy: policy, provider: provider, tags: tags,
				repository: project.Repository, pathErr: err,
			})
			continue
		}
		missing := !isProjectDir(localPath)
		displayPath := localPath
		if missing {
			displayPath += " (missing)"
		}
		rows = append(rows, listRow{
			name: displayName, path: displayPath,
			policy: policy, provider: provider, tags: tags,
			repository: project.Repository, missingDir: missing,
			rawPath: localPath,
		})
	}
	return rows, hasTags, errs
}

func enrichRowsWithGit(ctx context.Context, git *core.GitCommands, rows []listRow) {
	for i := range rows {
		r := &rows[i]
		if r.missingDir || r.pathErr != nil {
			continue
		}
		r.branch, _ = git.CurrentBranch(ctx, r.rawPath)
		syncSt, dirtySt, siErr := git.StatusInfo(ctx, r.rawPath)
		if siErr != nil {
			continue
		}
		r.syncAhead = syncSt.Ahead
		r.syncBehind = syncSt.Behind
		r.syncNoRemote = syncSt.NoRemote
		syms := dirtySt.Symbols()
		if syms != "—" {
			r.dirtySymbols = syms
		}
		r.hasExtended = true
	}
}

func filterListRows(rows []listRow, filterDirty, filterBehind bool) []listRow {
	filtered := rows[:0:0]
	for _, r := range rows {
		if r.missingDir || r.pathErr != nil || !r.hasExtended {
			continue
		}
		if filterDirty && r.dirtySymbols == "" {
			continue
		}
		if filterBehind && r.syncBehind == 0 {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func printListTable(rows []listRow, extended, hasTags bool, t *tracker) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	switch {
	case extended && hasTags:
		fmt.Fprintln(w, "NAME\tBRANCH\tSTATUS\tDIRTY\tPATH\tPOLICY\tPROVIDER\tTAGS")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.name, listBranch(r), listSync(t, r), listDirty(r), r.path, r.policy, r.provider, r.tags)
		}
	case extended:
		fmt.Fprintln(w, "NAME\tBRANCH\tSTATUS\tDIRTY\tPATH\tPOLICY\tPROVIDER")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.name, listBranch(r), listSync(t, r), listDirty(r), r.path, r.policy, r.provider)
		}
	case hasTags:
		fmt.Fprintln(w, "NAME\tPATH\tPOLICY\tPROVIDER\tTAGS")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.name, r.path, r.policy, r.provider, r.tags)
		}
	default:
		fmt.Fprintln(w, "NAME\tPATH\tPOLICY\tPROVIDER")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.name, r.path, r.policy, r.provider)
		}
	}
	w.Flush()
}

func doList(c *cli.Context) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, warnings, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	if !c.Bool("all") {
		projects = filterDisabled(projects)
	}

	extended := c.Bool("extended")
	filterDirty := c.Bool("dirty")
	filterBehind := c.Bool("behind")

	rows, hasTags, errs := buildListRows(projects)

	if extended || filterDirty || filterBehind {
		git, gitErr := core.NewGit(ui)
		if gitErr != nil {
			return exitErrorf(1, "Error loading git: %v", gitErr)
		}
		enrichRowsWithGit(context.Background(), git, rows)
		if filterDirty || filterBehind {
			rows = filterListRows(rows, filterDirty, filterBehind)
		}
	}

	if jsonMode {
		return printListJSON(rows, projects, warnings)
	}
	t := newTracker(0)
	printListTable(rows, extended, hasTags, t)
	return buildError(errs)
}

func listBranch(r listRow) string {
	if r.branch == "" {
		return "—"
	}
	return r.branch
}

func listSync(t *tracker, r listRow) string {
	if !r.hasExtended {
		return "—"
	}
	return t.colorSync(r.syncAhead, r.syncBehind, r.syncNoRemote)
}

func listDirty(r listRow) string {
	if r.dirtySymbols == "" {
		return "—"
	}
	return r.dirtySymbols
}

func printListJSON(rows []listRow, projects []gipProject, warnings []string) error {
	// Build a name → project index map for matching after rows may be filtered.
	projectByName := make(map[string]gipProject, len(projects))
	for _, p := range projects {
		projectByName[p.Name] = p
	}

	pjs := make([]listProjectJSON, 0, len(rows))
	for _, r := range rows {
		// Strip any display decoration from the name to recover the project name.
		pname := strings.TrimSuffix(r.name, " (disabled)")
		p, ok := projectByName[pname]
		if !ok {
			// Fallback: use whatever name is in the row.
			p.Name = r.name
			p.LocalPath = r.rawPath
			p.Repository = r.repository
		}
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}
		entry := listProjectJSON{
			Name:       p.Name,
			LocalPath:  p.LocalPath,
			Repository: r.repository,
			Policy:     r.policy,
			Provider:   r.provider,
			Tags:       tags,
			Missing:    r.missingDir,
			Disabled:   p.Disabled,
		}
		if r.branch != "" {
			entry.Branch = r.branch
		}
		if r.hasExtended {
			entry.Ahead = r.syncAhead
			entry.Behind = r.syncBehind
			entry.NoRemote = r.syncNoRemote
			if r.dirtySymbols != "" {
				entry.Dirty = r.dirtySymbols
			}
		}
		pjs = append(pjs, entry)
	}
	if warnings == nil {
		warnings = []string{}
	}
	out := listOutputJSON{
		Command:   "list",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Projects:  pjs,
		Warnings:  warnings,
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintf(os.Stdout, "%s\n", data)
	return nil
}

func doPull(c *cli.Context) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	args := c.Args().Slice()
	if len(args) > 0 {
		return exitErrorf(1, "Pull command does not accept any argument, found: %v", args)
	}
	all := c.Bool("all")
	ui.Confidentialf("%s PULL all? %t", configurationFile, all)
	projects, warnings, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
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

	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup

	filterDirty := c.Bool("dirty")
	filterBehind := c.Bool("behind")

	for _, project := range projects {
		project := project
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			pullOne(c, git, project, all, t, filterDirty, filterBehind)
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("pull", warnings)
	} else {
		t.printSummary(c.Bool("errors-last"))
	}
	return buildError(t.errors())
}

func pullOne(c *cli.Context, git *core.GitCommands, project gipProject, all bool, t *tracker, filterDirty, filterBehind bool) {
	if project.pullNever() {
		ui.Confidentialf("Skip %s : pull policy never", project.Name)
		if noopMode {
			t.printNoop("%s → SKIPPED  (pull_policy: never)", project.Name)
		}
		t.record(opResult{project: project.Name, status: opSkipped, reason: "pull_policy: never"})
		return
	}

	line, err := projectPath(project.LocalPath)
	if err != nil {
		t.record(opResult{project: project.Name, status: opError, err: err})
		return
	}
	if !isProjectDir(line) {
		pullMissingDir(c, git, project, line, all, t)
		return
	}
	if noopMode {
		t.printNoop("%s → git pull  (in %s)", project.Name, line)
		t.record(opResult{project: project.Name, localPath: line, status: opOK})
		return
	}
	ctx, cancel := opContext(c)
	defer cancel()
	if skip, reason, _ := filterByGitState(ctx, git, line, filterDirty, filterBehind); skip {
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
		return
	}
	if err = git.Pull(ctx, line); err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
	} else {
		t.record(opResult{project: project.Name, localPath: line, status: opOK})
	}
}

func pullMissingDir(c *cli.Context, git *core.GitCommands, project gipProject, line string, all bool, t *tracker) {
	t.withOutput(func() { warnMissingDir(line) })
	if !(all || project.pullAlways()) {
		if noopMode {
			t.printNoop("%s → SKIPPED  (directory missing)", project.Name)
		}
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: "local dir missing"})
		return
	}
	if noopMode {
		t.printNoop("%s → git clone %s %s", project.Name, project.Repository, line)
		t.record(opResult{project: project.Name, localPath: line, status: opOK})
		return
	}
	ctx, cancel := opContext(c)
	defer cancel()
	if err := git.Clone(ctx, project.Repository, line); err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
	} else {
		t.record(opResult{project: project.Name, localPath: line, status: opOK})
	}
}

func doFetch(c *cli.Context) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, warnings, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	projects = filterDisabled(projects)
	t := newTracker(len(projects))
	git, err := core.NewGit(ui, core.WithSharedOutput(t.sharedOutput()))
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}

	filterDirty := c.Bool("dirty")
	filterBehind := c.Bool("behind")

	jobs, err := resolveJobs(c)
	if err != nil {
		return exitErrorf(1, "%v", err)
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

			if project.pullNever() {
				ui.Confidentialf("Skip %s : pull policy never", project.Name)
				if noopMode {
					t.printNoop("%s → SKIPPED  (pull_policy: never)", project.Name)
				}
				t.record(opResult{project: project.Name, status: opSkipped, reason: "pull_policy: never"})
				return
			}

			line, err := projectPath(project.LocalPath)
			if err != nil {
				t.record(opResult{project: project.Name, status: opError, err: err})
				return
			}
			if !isProjectDir(line) {
				t.withOutput(func() { warnMissingDir(line) })
				if noopMode {
					t.printNoop("%s → SKIPPED  (directory missing)", project.Name)
				}
				t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: "local dir missing"})
				return
			}
			if noopMode {
				t.printNoop("%s → git fetch --all --prune  (in %s)", project.Name, line)
				t.record(opResult{project: project.Name, localPath: line, status: opOK})
				return
			}
			ctx, cancel := opContext(c)
			defer cancel()
			if skip, reason, _ := filterByGitState(ctx, git, line, filterDirty, filterBehind); skip {
				t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
				return
			}
			if err = git.Fetch(ctx, line); err != nil {
				t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
			} else {
				t.record(opResult{project: project.Name, localPath: line, status: opOK})
			}
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("fetch", warnings)
	} else {
		t.printSummary(c.Bool("errors-last"))
	}
	return buildError(t.errors())
}

type branchEntry struct {
	name         string
	path         string
	branch       string
	err          error
	syncAhead    int
	syncBehind   int
	syncNoRemote bool
	dirtySymbols string // "+*?$" or "" when not collected/clean
	commitMsg    string
	commitDate   string
	hasExtended  bool // whether extended info was successfully collected
}

// filterByGitState checks dirty/behind filter flags against a repo's StatusInfo.
// Returns (shouldSkip, skipReason, error). On error the repo is included (not skipped).
func filterByGitState(ctx context.Context, git *core.GitCommands, dirpath string, filterDirty, filterBehind bool) (bool, string, error) {
	if !filterDirty && !filterBehind {
		return false, "", nil
	}
	sync, dirty, err := git.StatusInfo(ctx, dirpath)
	if err != nil {
		return false, "", err
	}
	if filterDirty && !dirty.IsDirty() {
		return true, "not dirty", nil
	}
	if filterBehind && sync.Behind == 0 {
		return true, "not behind", nil
	}
	return false, "", nil
}

// enrichBranchEntry populates extended fields on entry from git status. Returns true if the project should be skipped.
func enrichBranchEntry(ctx context.Context, git *core.GitCommands, entry *branchEntry, projectName, line string, filterDirty, filterBehind, extended bool, t *tracker) bool {
	syncSt, dirtySt, siErr := git.StatusInfo(ctx, line)
	if siErr != nil {
		return false
	}
	entry.syncAhead = syncSt.Ahead
	entry.syncBehind = syncSt.Behind
	entry.syncNoRemote = syncSt.NoRemote
	if syms := dirtySt.Symbols(); syms != "—" {
		entry.dirtySymbols = syms
	}
	entry.hasExtended = true
	if filterDirty && !dirtySt.IsDirty() {
		t.record(opResult{project: projectName, localPath: line, status: opSkipped, reason: "not dirty"})
		return true
	}
	if filterBehind && syncSt.Behind == 0 {
		t.record(opResult{project: projectName, localPath: line, status: opSkipped, reason: "not behind"})
		return true
	}
	if extended {
		msg, date, _ := git.LastCommit(ctx, line)
		entry.commitMsg = msg
		entry.commitDate = date
	}
	return false
}

func processBranchProject(project gipProject, git *core.GitCommands, t *tracker, entries *[]branchEntry, mu *sync.Mutex, needsStatusInfo, filterDirty, filterBehind, extended bool, c *cli.Context) {
	entry := branchEntry{name: project.Name}
	line, err := projectPath(project.LocalPath)
	if err != nil {
		entry.err = err
		t.record(opResult{project: project.Name, status: opError, err: err})
		mu.Lock()
		*entries = append(*entries, entry)
		mu.Unlock()
		return
	}
	entry.path = line
	if !isProjectDir(line) {
		entry.branch = "(missing)"
		if noopMode {
			t.printNoop("%s → SKIPPED  (directory missing)", project.Name)
		}
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: "local dir missing"})
		if !filterDirty && !filterBehind {
			mu.Lock()
			*entries = append(*entries, entry)
			mu.Unlock()
		}
		return
	}
	if noopMode {
		t.printNoop("%s → git rev-parse --abbrev-ref HEAD  (in %s)", project.Name, line)
		entry.branch = "(noop)"
		t.record(opResult{project: project.Name, localPath: line, status: opOK, branch: "(noop)"})
		mu.Lock()
		*entries = append(*entries, entry)
		mu.Unlock()
		return
	}
	ctx, cancel := opContext(c)
	defer cancel()
	entry.branch, entry.err = git.CurrentBranch(ctx, line)
	if entry.err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: entry.err})
		mu.Lock()
		*entries = append(*entries, entry)
		mu.Unlock()
		return
	}
	if needsStatusInfo {
		if enrichBranchEntry(ctx, git, &entry, project.Name, line, filterDirty, filterBehind, extended, t) {
			return
		}
	}
	res := opResult{
		project: project.Name, localPath: line, status: opOK,
		branch: entry.branch, commitMsg: entry.commitMsg, commitDate: entry.commitDate,
		dirtySymbols: entry.dirtySymbols,
	}
	if entry.hasExtended {
		res.syncAhead = entry.syncAhead
		res.syncBehind = entry.syncBehind
		res.syncNoRemote = entry.syncNoRemote
		res.syncSet = true
	}
	t.record(res)
	mu.Lock()
	*entries = append(*entries, entry)
	mu.Unlock()
}

func printBranchTable(entries []branchEntry, extended bool, t *tracker) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if extended {
		fmt.Fprintln(w, "NAME\tBRANCH\tSTATUS\tDIRTY\tCOMMIT\tDATE\tPATH")
		for _, e := range entries {
			branch := e.branch
			if e.err != nil {
				branch = fmt.Sprintf("ERROR: %v", e.err)
			}
			status, dirty, commit, date := "—", "—", "—", "—"
			if e.hasExtended {
				status = t.colorSync(e.syncAhead, e.syncBehind, e.syncNoRemote)
				if e.dirtySymbols != "" {
					dirty = e.dirtySymbols
				}
				if e.commitMsg != "" {
					commit = e.commitMsg
					date = e.commitDate
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", e.name, branch, status, dirty, commit, date, e.path)
		}
	} else {
		fmt.Fprintln(w, "NAME\tBRANCH\tPATH")
		for _, e := range entries {
			branch := e.branch
			if e.err != nil {
				branch = fmt.Sprintf("ERROR: %v", e.err)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", e.name, branch, e.path)
		}
	}
	w.Flush()
}

func doBranch(c *cli.Context) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, warnings, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	projects = filterDisabled(projects)
	t := newTracker(len(projects))
	git, err := core.NewGit(ui, core.WithSharedOutput(t.sharedOutput()))
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}
	extended := c.Bool("extended")
	filterDirty := c.Bool("dirty")
	filterBehind := c.Bool("behind")
	needsStatusInfo := extended || filterDirty || filterBehind
	jobs, err := resolveJobs(c)
	if err != nil {
		return exitErrorf(1, "%v", err)
	}
	var entriesMu sync.Mutex
	entries := make([]branchEntry, 0, len(projects))
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for _, project := range projects {
		project := project
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			processBranchProject(project, git, t, &entries, &entriesMu, needsStatusInfo, filterDirty, filterBehind, extended, c)
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("branch", warnings)
	} else {
		printBranchTable(entries, extended, t)
		t.printSummary(c.Bool("errors-last"))
	}
	return buildError(t.errors())
}

func execProject(project gipProject, runner *core.CommandRunner, git *core.GitCommands, cmdName string, cmdArgs []string, filterDirty, filterBehind bool, t *tracker, c *cli.Context) {
	line, err := projectPath(project.LocalPath)
	if err != nil {
		t.record(opResult{project: project.Name, status: opError, err: err})
		return
	}
	if !isProjectDir(line) {
		t.withOutput(func() { warnMissingDir(line) })
		if noopMode {
			t.printNoop("%s → SKIPPED  (directory missing)", project.Name)
		}
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: "not a project dir"})
		return
	}
	if noopMode {
		t.printNoop("%s → %s %s  (in %s)", project.Name, cmdName, strings.Join(cmdArgs, " "), line)
		t.record(opResult{project: project.Name, localPath: line, status: opOK})
		return
	}
	ctx, cancel := opContext(c)
	defer cancel()
	if git != nil {
		if skip, reason, _ := filterByGitState(ctx, git, line, filterDirty, filterBehind); skip {
			t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
			return
		}
	}
	if err = runner.Run(ctx, line, cmdName, cmdArgs); err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
	} else {
		t.record(opResult{project: project.Name, localPath: line, status: opOK})
	}
}

func doExec(c *cli.Context) error {
	args := c.Args().Slice()
	if len(args) == 0 {
		return exitErrorf(1, "exec requires a command: gip exec -- <cmd> [args...]")
	}
	cmdName := args[0]
	cmdArgs := args[1:]
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, warnings, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	projects = filterDisabled(projects)
	t := newTracker(len(projects))
	runner := core.NewCommandRunner(ui, core.WithSharedOutputRunner(t.sharedOutput()))
	filterDirty := c.Bool("dirty")
	filterBehind := c.Bool("behind")
	var git *core.GitCommands
	if filterDirty || filterBehind {
		git, err = core.NewGit(ui, core.WithSharedOutput(t.sharedOutput()))
		if err != nil {
			return exitErrorf(1, "Error loading git: %v", err)
		}
	}
	jobs, err := resolveJobs(c)
	if err != nil {
		return exitErrorf(1, "%v", err)
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
			execProject(project, runner, git, cmdName, cmdArgs, filterDirty, filterBehind, t, c)
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("exec", warnings)
	} else {
		t.printSummary(c.Bool("errors-last"))
	}
	return buildError(t.errors())
}

// initEntry is the shape written to the generated config file.
type initEntry struct {
	Name       string `yaml:"name"`
	Repository string `yaml:"repository"`
	LocalPath  string `yaml:"local_path"`
}

func doInit(c *cli.Context) error {
	scanDir := c.Args().First()
	if scanDir == "" {
		scanDir = "."
	}
	scanDir, err := projectPath(scanDir)
	if err != nil {
		return exitErrorf(1, "Error expanding path %s: %v", scanDir, err)
	}
	scanDir, err = filepath.Abs(scanDir)
	if err != nil {
		return exitErrorf(1, "Error resolving path %s: %v", scanDir, err)
	}

	ui.Lifecyclef("Scanning %s ...", scanDir)

	repos, err := scanForRepos(scanDir, c.Int("depth"))
	if err != nil {
		return exitErrorf(1, "Error scanning directory: %v", err)
	}

	var entries []initEntry
	for _, repoPath := range repos {
		name := filepath.Base(repoPath)
		url, err := getOriginURL(repoPath)
		if err != nil {
			ui.Warnf("  skip %s (no origin remote)", name)
			continue
		}
		ui.Lifecyclef("  found %s  %s", name, url)
		entries = append(entries, initEntry{
			Name:       name,
			Repository: url,
			LocalPath:  repoPath,
		})
	}

	if len(entries) == 0 {
		ui.Lifecyclef("No repositories with an origin remote found in %s", scanDir)
		return nil
	}

	outputPath := c.String("output")
	if outputPath == "" {
		outputPath = filepath.Join(".", configFileBaseName)
	}

	if _, statErr := os.Stat(outputPath); statErr == nil && !c.Bool("force") {
		fmt.Printf("%s already exists. Overwrite? [y/N]: ", outputPath)
		var response string
		fmt.Scanln(&response) //nolint:errcheck
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(response)), "y") {
			ui.Lifecyclef("Aborted.")
			return nil
		}
	}

	data, err := yaml.Marshal(entries)
	if err != nil {
		return exitErrorf(1, "Error generating YAML: %v", err)
	}
	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		return exitErrorf(1, "Error writing %s: %v", outputPath, err)
	}

	ui.Lifecyclef("Configuration written to %s (%d repositories)", outputPath, len(entries))
	return nil
}

// scanForRepos walks root up to maxDepth levels deep and returns the paths of
// all directories that contain a .git entry (file or directory).
func scanForRepos(root string, maxDepth int) ([]string, error) {
	rootDepth := len(strings.Split(filepath.Clean(root), string(filepath.Separator)))
	var repos []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		depth := len(strings.Split(filepath.Clean(path), string(filepath.Separator))) - rootDepth
		if depth > maxDepth {
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			repos = append(repos, path)
			return filepath.SkipDir
		}
		return nil
	})
	return repos, err
}

// getOriginURL returns the URL of the "origin" remote for the repo at repoPath.
func getOriginURL(repoPath string) (string, error) {
	out, err := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("no origin remote")
	}
	return strings.TrimSpace(string(out)), nil
}

func warnMissingDir(dir string) {
	if ignoreMissingDirs {
		return
	}
	ui.Warnf("%s (not a project dir)", dir)
}

func exitErrorf(exitCode int, template string, args ...interface{}) error {
	return exitError(exitCode, fmt.Sprintf(template, args...))
}

func exitError(exitCode int, message string) error {
	return cli.Exit(message, exitCode)
}

func configurationFilePath(c *cli.Context) (string, error) {
	if c.String("f") != "" {
		abs, err := filepath.Abs(c.String("f"))
		if err != nil {
			return "", err
		}
		return filepath.FromSlash(filepath.Clean(abs)), nil
	}
	return defaultConfigurationFilePath()
}
