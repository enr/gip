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
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/enr/gip/lib/core"

	"github.com/urfave/cli/v2"
	yaml "gopkg.in/yaml.v3"
)

var tagFlag = &cli.StringSliceFlag{
	Name:  "tag",
	Usage: "filter projects by tag (comma-separated and/or repeated, OR logic): --tag work,js or --tag work --tag js",
}

// tagFilterValue joins the (possibly repeated, possibly comma-separated)
// --tag flag values into the single comma-separated string filterByTag expects.
func tagFilterValue(c *cli.Context) string {
	return strings.Join(c.StringSlice("tag"), ",")
}

var errorsLastFlag = &cli.BoolFlag{
	Name:  "errors-last",
	Usage: "print a grouped error section after the summary",
}

var showSkippedFlag = &cli.BoolFlag{
	Name:  "show-skipped",
	Usage: "print the skipped projects, with the reason, after the summary",
}

var extendedFlag = &cli.BoolFlag{
	Name:  "extended",
	Usage: "show dirty indicators in addition to branch and sync status",
}

var dirtyFilterFlag = &cli.BoolFlag{
	Name:  "dirty",
	Usage: "only include repos with uncommitted changes (staged, unstaged, or untracked)",
}

var behindFilterFlag = &cli.BoolFlag{
	Name:  "behind",
	Usage: "only include repos behind their upstream (includes diverged); requires up-to-date remote refs — run gip fetch first",
}

var aheadFilterFlag = &cli.BoolFlag{
	Name:  "ahead",
	Usage: "only include repos ahead of their upstream (includes diverged); requires up-to-date remote refs — run gip fetch first",
}

var unsyncedFilterFlag = &cli.BoolFlag{
	Name:  "unsynced",
	Usage: "only include repos out of sync with upstream (ahead or behind); alias for --behind --ahead; requires up-to-date remote refs — run gip fetch first",
}

const maxJobs = 32

var parallelFlags = []cli.Flag{
	&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}, Value: 4, Usage: fmt.Sprintf("maximum number of repos to operate on concurrently (1-%d)", maxJobs)},
	&cli.IntFlag{Name: "timeout", Aliases: []string{"t"}, Value: 0, Usage: "per-operation timeout in seconds (0 = no timeout)"},
	tagFlag,
	errorsLastFlag,
	showSkippedFlag,
	dirtyFilterFlag,
	behindFilterFlag,
	aheadFilterFlag,
	unsyncedFilterFlag,
}

func bashCompleteWithFlags(flags []cli.Flag) cli.BashCompleteFunc {
	return func(c *cli.Context) {
		partial := ""
		args := os.Args
		if len(args) >= 2 {
			prev := args[len(args)-2]
			if strings.HasPrefix(prev, "-") {
				partial = strings.TrimLeft(prev, "-")
			}
		}
		for _, flag := range flags {
			for _, name := range flag.Names() {
				if len(name) > 1 && strings.HasPrefix(name, partial) {
					fmt.Fprintf(c.App.Writer, "--%s\n", name)
				}
			}
		}
	}
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
	&commandReleases,
	&commandExec,
	&commandCheck,
	&commandInit,
	&commandCompletion,
}

var commandStatus = cli.Command{
	Name:         "status",
	Aliases:      []string{"s"},
	Usage:        "show modified files in projects",
	Description:  `Prints modified files.`,
	Action:       doStatus,
	Flags:        parallelFlags,
	BashComplete: bashCompleteWithFlags(parallelFlags),
}

var commandStatusFull = cli.Command{
	Name:         "statusfull",
	Aliases:      []string{"sf"},
	Usage:        "show modified and new files in projects",
	Description:  `Prints modified files and new ones.`,
	Action:       doStatusFull,
	Flags:        parallelFlags,
	BashComplete: bashCompleteWithFlags(parallelFlags),
}

var listFlags = []cli.Flag{
	tagFlag,
	&cli.BoolFlag{Name: "all", Aliases: []string{"a"}, Usage: "include disabled projects in the list"},
	extendedFlag,
	dirtyFilterFlag,
	behindFilterFlag,
	aheadFilterFlag,
	unsyncedFilterFlag,
	&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}, Value: 4, Usage: fmt.Sprintf("maximum number of repos to query concurrently (1-%d)", maxJobs)},
	&cli.IntFlag{Name: "timeout", Aliases: []string{"t"}, Value: 0, Usage: "per-repo git status timeout in seconds (0 = no timeout)"},
}

var commandList = cli.Command{
	Name:         "list",
	Aliases:      []string{"ls"},
	Usage:        "list registered projects",
	Description:  `List projects in a table with name, branch, sync status, path, policy, provider and tags.`,
	Action:       doList,
	Flags:        listFlags,
	BashComplete: bashCompleteWithFlags(listFlags),
}

var pullFlags = append(parallelFlags,
	&cli.BoolFlag{Name: "all", Aliases: []string{"a"}, Usage: `Pull all registered projects doing a checkout if needed. Otherwise only the projects already present are updated.`},
	&cli.BoolFlag{Name: "force", Usage: "pull even when the working tree has uncommitted changes (by default such repos are skipped)"},
)

var commandPull = cli.Command{
	Name:         "pull",
	Usage:        "update projects from remote repositories",
	Description:  `Pull projects`,
	Action:       doPull,
	Flags:        pullFlags,
	BashComplete: bashCompleteWithFlags(pullFlags),
}

var commandFetch = cli.Command{
	Name:         "fetch",
	Usage:        "fetch remote refs for all projects without merging",
	Description:  `Executes "git fetch --all --prune" for each project. Projects with pull_policy "never" are skipped.`,
	Action:       doFetch,
	Flags:        parallelFlags,
	BashComplete: bashCompleteWithFlags(parallelFlags),
}

var branchFlags = append(parallelFlags, extendedFlag)

var commandBranch = cli.Command{
	Name:         "branch",
	Aliases:      []string{"br"},
	Usage:        "show current branch for each project",
	Description:  `Prints the current branch (or "(detached)" for a detached HEAD) for every project in a table.`,
	Action:       doBranch,
	Flags:        branchFlags,
	BashComplete: bashCompleteWithFlags(branchFlags),
}

var commandExec = cli.Command{
	Name:  "exec",
	Usage: "execute an arbitrary command in each project directory",
	Description: `Execute an arbitrary command in each project directory.
Use -- to separate gip flags from the command and its arguments:

   gip exec -- git fetch --prune
   gip exec -j 8 -- make test`,
	Action:       doExec,
	Flags:        parallelFlags,
	BashComplete: bashCompleteWithFlags(parallelFlags),
}

var initFlags = []cli.Flag{
	&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "output file path (default: ./.gip)"},
	&cli.BoolFlag{Name: "force", Usage: "overwrite existing config without prompting"},
	&cli.IntFlag{Name: "depth", Value: 5, Usage: "maximum directory scan depth"},
}

var commandInit = cli.Command{
	Name:         "init",
	Usage:        "scan a directory for git repositories and generate configuration",
	Description:  `Scans a directory recursively for git repositories and writes a gip configuration file.`,
	Action:       doInit,
	Flags:        initFlags,
	BashComplete: bashCompleteWithFlags(initFlags),
}

var commandCompletion = cli.Command{
	Name:  "completion",
	Usage: "generate shell completion script",
	Description: `Output a shell completion script for the specified shell.

Supported shells: bash, zsh, fish

Examples:
   source <(gip completion bash)
   source <(gip completion zsh)
   gip completion fish > ~/.config/fish/completions/gip.fish`,
	Action: doCompletion,
}

const bashCompletionScript = `# bash completion for gip — source <(gip completion bash)
_gip_completion() {
    local cur
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    # Words before the current partial word (exclude gip itself at index 0)
    local -a base_args=("${COMP_WORDS[@]:1:$((COMP_CWORD-1))}")
    # Build trigger args: add cur as last arg so our BashComplete sees it as
    # the prefix to filter, but skip "--" (urfave disables completion on --)
    local -a trigger_args=("${base_args[@]}")
    if [[ -n "$cur" && "$cur" != "--" ]]; then
        trigger_args+=("$cur")
    fi
    local coms
    coms=$(gip "${trigger_args[@]}" --generate-bash-completion 2>/dev/null)
    while IFS='' read -r line; do COMPREPLY+=("$line"); done < <(compgen -W "$coms" -- "$cur")
    return 0
}
complete -F _gip_completion gip
`

const zshCompletionScript = `# zsh completion for gip — source <(gip completion zsh)
_gip() {
    local -a completions
    local cur="${words[CURRENT]}"
    # Words before current partial (1-indexed; words[1] is gip, skip it)
    local -a partial_args
    partial_args=("${words[@]:2:$((CURRENT-2))}")
    # Add current partial as trigger (skip "--" which disables urfave completion)
    local -a trigger_args=("${partial_args[@]}")
    if [[ -n "$cur" && "$cur" != "--" ]]; then
        trigger_args+=("$cur")
    fi
    completions=($(gip "${trigger_args[@]}" --generate-bash-completion 2>/dev/null))
    compadd -a completions
}
compdef _gip gip
`

func doCompletion(c *cli.Context) error {
	shell := c.Args().First()
	if shell == "" {
		return exitErrorf(1, "completion requires a shell argument: bash, zsh, or fish")
	}
	switch shell {
	case "fish":
		script, err := c.App.ToFishCompletion()
		if err != nil {
			return exitErrorf(1, "Error generating fish completion: %v", err)
		}
		fmt.Fprint(os.Stdout, script)
	case "bash":
		fmt.Fprint(os.Stdout, bashCompletionScript)
	case "zsh":
		fmt.Fprint(os.Stdout, zshCompletionScript)
	default:
		return exitErrorf(1, "unsupported shell %q: supported shells are bash, zsh, fish", shell)
	}
	return nil
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
	projects = filterByTag(projects, tagFilterValue(c))
	projects = filterDisabled(projects)
	t := newTracker(len(projects))
	git, err := core.NewGit(ui, core.WithSharedOutput(t.sharedOutput()))
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}
	filter := newGitStateFilter(c)

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
			doStatusProject(c, git, t, project, untracked, untrackedFlag, filter)
			statusExtraBranches(c, git, t, project, filter)
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("status", warnings)
	} else {
		t.printSummary(c.Bool("errors-last"), c.Bool("show-skipped"))
	}
	return buildError(t.errors())
}

func doStatusProject(c *cli.Context, git *core.GitCommands, t *tracker, project gipProject, untracked bool, untrackedFlag string, filter gitStateFilter) {
	line, err := project.resolvedPath()
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

	if untracked && !filter.active() {
		statusProjectFull(ctx, git, t, project, line)
		return
	}
	reason, err := filter.check(ctx, git, line)
	if err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
		return
	}
	if reason != "" {
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
		return
	}
	if err = git.Status(ctx, line, untracked); err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
	} else {
		t.record(opResult{project: project.Name, localPath: line, status: opOK})
	}
}

// statusProjectFull handles "status sf": shows unsynced repos (ahead/behind/no-remote) together with dirty ones.
func statusProjectFull(ctx context.Context, git *core.GitCommands, t *tracker, project gipProject, line string) {
	st, err := readRepoState(ctx, git, line)
	if err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
		return
	}
	isUnsynced := st.sync.NoRemote || st.sync.Behind > 0 || st.sync.Ahead > 0
	if !st.dirty.IsDirty() && !isUnsynced {
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: "clean and in sync"})
		return
	}
	res := opResult{project: project.Name, localPath: line, state: &st}
	if st.dirty.IsDirty() {
		if err = git.Status(ctx, line, true); err != nil {
			res.status = opError
			res.err = err
		} else {
			res.status = opOK
		}
	} else {
		t.withOutput(func() {
			ui.Lifecyclef("%s  %s", line, t.colorSync(st.sync, false))
		})
		res.status = opOK
	}
	t.record(res)
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

// listRow is one project of the list table.
type listRow struct {
	project gipProject
	path    string // resolved local_path; empty when resolution failed
	missing bool
	state   *repoState
	err     error
}

func buildListRows(projects []gipProject) []listRow {
	rows := make([]listRow, 0, len(projects))
	for _, project := range projects {
		row := listRow{project: project}
		row.path, row.err = project.resolvedPath()
		row.missing = row.err == nil && !isProjectDir(row.path)
		rows = append(rows, row)
	}
	return rows
}

// readListStates reads the repository state of every present project, in
// parallel, drawing t's progress bar as each one completes: on slow
// filesystems (e.g. /mnt/c under WSL) git status can take a long time and
// the table is only printed at the end.
func readListStates(c *cli.Context, git *core.GitCommands, rows []listRow, jobs int, t *tracker) {
	for _, r := range rows {
		if !r.missing && r.err == nil {
			t.expect(1)
		}
	}
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for i := range rows {
		r := &rows[i]
		if r.missing || r.err != nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			defer t.record(opResult{project: r.project.Name})
			// --timeout applies to each repo, not to the whole batch.
			ctx, cancel := opContext(c)
			defer cancel()
			st, err := readRepoState(ctx, git, r.path)
			if err != nil {
				r.err = err
				return
			}
			r.state = &st
		}()
	}
	wg.Wait()
	t.clearProgress()
}

func listErrors(rows []listRow) map[string]error {
	errs := map[string]error{}
	for _, r := range rows {
		if r.err != nil {
			errs[r.project.Name] = r.err
		}
	}
	return errs
}

// filterListRows keeps the rows whose state is known and matches filter.
func filterListRows(rows []listRow, filter gitStateFilter) []listRow {
	filtered := rows[:0:0]
	for _, r := range rows {
		if r.state != nil && filter.skipReason(*r.state) == "" {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func (r listRow) policy() string {
	if r.project.PullPolicy == "" {
		return "default"
	}
	return r.project.PullPolicy
}

// ansiRe matches ANSI CSI escape sequences (e.g. color codes from lipgloss).
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// visualLen returns the visible character width of s, ignoring ANSI escape sequences.
func visualLen(s string) int {
	return utf8.RuneCountInString(ansiRe.ReplaceAllString(s, ""))
}

// renderTable prints rows with consistent column alignment, correctly handling
// ANSI color codes (which have byte length > visual width).
// gap is the number of spaces between columns.
func renderTable(rows [][]string, gap int) {
	if len(rows) == 0 {
		return
	}
	cols := len(rows[0])
	widths := make([]int, cols)
	for _, row := range rows {
		for j := range widths {
			if j < len(row) {
				if vl := visualLen(row[j]); vl > widths[j] {
					widths[j] = vl
				}
			}
		}
	}
	for _, row := range rows {
		var sb strings.Builder
		for j, cell := range row {
			sb.WriteString(cell)
			if j < cols-1 {
				sb.WriteString(strings.Repeat(" ", widths[j]-visualLen(cell)+gap))
			}
		}
		fmt.Fprintln(os.Stdout, sb.String())
	}
}

func printListTable(rows []listRow, extended bool, t *tracker) {
	hasTags := false
	for _, r := range rows {
		hasTags = hasTags || len(r.project.Tags) > 0
	}
	header := []string{"NAME", "BRANCH", "STATUS"}
	if extended {
		header = append(header, "DIRTY")
	}
	header = append(header, "PATH", "POLICY", "PROVIDER")
	if hasTags {
		header = append(header, "TAGS")
	}
	table := [][]string{header}
	for _, r := range rows {
		name := r.project.Name
		if r.project.Disabled {
			name += " (disabled)"
		}
		branch, status, dirty := "—", "—", "—"
		switch {
		case r.state != nil:
			branch = cell(r.state.sync.Branch)
			status = t.colorSync(r.state.sync, r.state.dirty.IsDirty())
			dirty = cell(r.state.dirty.Symbols())
		case r.err != nil:
			status = t.applyStyle(styleErr, "ERROR")
		}
		path := r.path
		switch {
		case r.path == "":
			path = r.project.LocalPath + " (ERROR)"
		case r.missing:
			path += " (missing)"
		}
		row := []string{name, branch, status}
		if extended {
			row = append(row, dirty)
		}
		row = append(row, path, r.policy(), cell(r.project.repoProvider()))
		if hasTags {
			row = append(row, cell(strings.Join(r.project.Tags, ", ")))
		}
		table = append(table, row)
	}
	renderTable(table, 2)
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
	projects = filterByTag(projects, tagFilterValue(c))
	if !c.Bool("all") {
		projects = filterDisabled(projects)
	}
	filter := newGitStateFilter(c)
	jobs, err := resolveJobs(c)
	if err != nil {
		return exitErrorf(1, "%v", err)
	}
	git, err := core.NewGit(ui)
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}

	rows := buildListRows(projects)
	t := newTracker(0)
	readListStates(c, git, rows, jobs, t)
	errs := listErrors(rows)
	if filter.active() {
		rows = filterListRows(rows, filter)
	}
	if jsonMode {
		printListJSON(rows, warnings)
	} else {
		printListTable(rows, c.Bool("extended"), t)
	}
	return buildError(errs)
}

func printListJSON(rows []listRow, warnings []string) {
	pjs := make([]listProjectJSON, 0, len(rows))
	for _, r := range rows {
		p := r.project
		entry := listProjectJSON{
			Name:       p.Name,
			LocalPath:  r.path,
			Repository: p.Repository,
			Policy:     r.policy(),
			Provider:   p.repoProvider(),
			Tags:       p.Tags,
			Missing:    r.missing,
			Disabled:   p.Disabled,
		}
		if entry.LocalPath == "" {
			entry.LocalPath = p.LocalPath
		}
		if entry.Tags == nil {
			entry.Tags = []string{}
		}
		if r.err != nil {
			entry.Error = r.err.Error()
		}
		if r.state != nil {
			entry.Branch = r.state.sync.Branch
			entry.Ahead = r.state.sync.Ahead
			entry.Behind = r.state.sync.Behind
			entry.NoRemote = r.state.sync.NoRemote
			entry.Dirty = r.state.dirty.Symbols()
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
	if c.Bool("dirty") && !c.Bool("force") {
		return exitErrorf(1, "--dirty selects only repositories with uncommitted changes, which pull skips without --force: add --force or drop --dirty")
	}
	all := c.Bool("all")
	ui.Confidentialf("%s PULL all? %t", configurationFile, all)
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

	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup

	filter := newGitStateFilter(c)

	for _, project := range projects {
		project := project
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			pullOne(c, git, project, all, t, filter)
			pullExtraBranches(c, git, t, project, filter)
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("pull", warnings)
	} else {
		t.printSummary(c.Bool("errors-last"), c.Bool("show-skipped"))
	}
	return buildError(t.errors())
}

func pullOne(c *cli.Context, git *core.GitCommands, project gipProject, all bool, t *tracker, filter gitStateFilter) {
	if project.pullNever() {
		ui.Confidentialf("Skip %s : pull policy never", project.Name)
		if noopMode {
			t.printNoop("%s → SKIPPED  (pull_policy: never)", project.Name)
		}
		t.record(opResult{project: project.Name, status: opSkipped, reason: "pull_policy: never"})
		return
	}

	line, err := project.resolvedPath()
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
	force := c.Bool("force")
	// Without --force a pull happens only on a tree known to be clean: an
	// unreadable state is reported, never pulled blind.
	if filter.active() || !force {
		st, err := readRepoState(ctx, git, line)
		if err != nil {
			t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
			return
		}
		if reason := filter.skipReason(st); reason != "" {
			t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
			return
		}
		if !force && st.dirty.IsDirty() {
			t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: "uncommitted changes (use --force)"})
			return
		}
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
	projects = filterByTag(projects, tagFilterValue(c))
	projects = filterDisabled(projects)
	t := newTracker(len(projects))
	git, err := core.NewGit(ui, core.WithSharedOutput(t.sharedOutput()))
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}

	filter := newGitStateFilter(c)

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
			fetchOne(c, git, project, t, filter)
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("fetch", warnings)
	} else {
		t.printSummary(c.Bool("errors-last"), c.Bool("show-skipped"))
	}
	return buildError(t.errors())
}

// fetchOne fetches a single project, honouring pull_policy and the git state
// filters.
func fetchOne(c *cli.Context, git *core.GitCommands, project gipProject, t *tracker, filter gitStateFilter) {
	if project.pullNever() {
		ui.Confidentialf("Skip %s : pull policy never", project.Name)
		if noopMode {
			t.printNoop("%s → SKIPPED  (pull_policy: never)", project.Name)
		}
		t.record(opResult{project: project.Name, status: opSkipped, reason: "pull_policy: never"})
		return
	}

	line, err := project.resolvedPath()
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
	reason, err := filter.check(ctx, git, line)
	if err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
		return
	}
	if reason != "" {
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
		return
	}
	if err = git.Fetch(ctx, line); err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
	} else {
		t.record(opResult{project: project.Name, localPath: line, status: opOK})
	}
}

type branchEntry struct {
	name       string
	path       string
	branch     string
	err        error
	state      *repoState // nil unless --extended or a git-state filter is set
	commitMsg  string
	commitDate string
}

// branchProject collects the branch of one project and records its outcome.
// It returns ok=false when the project must not appear in the table.
func branchProject(c *cli.Context, git *core.GitCommands, t *tracker, project gipProject, filter gitStateFilter, extended bool) (entry branchEntry, ok bool) {
	entry = branchEntry{name: project.Name}
	line, err := project.resolvedPath()
	if err != nil {
		entry.err = err
		t.record(opResult{project: project.Name, status: opError, err: err})
		return entry, true
	}
	entry.path = line
	fail := func(err error) (branchEntry, bool) {
		entry.err = err
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
		return entry, true
	}
	if !isProjectDir(line) {
		entry.branch = "(missing)"
		t.withOutput(func() { warnMissingDir(line) })
		if noopMode {
			t.printNoop("%s → SKIPPED  (directory missing)", project.Name)
		}
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: "local dir missing"})
		return entry, !filter.active()
	}
	if noopMode {
		t.printNoop("%s → git rev-parse --abbrev-ref HEAD  (in %s)", project.Name, line)
		entry.branch = "(noop)"
		t.record(opResult{project: project.Name, localPath: line, status: opOK, branch: "(noop)"})
		return entry, true
	}
	ctx, cancel := opContext(c)
	defer cancel()
	if entry.branch, err = git.CurrentBranch(ctx, line); err != nil {
		return fail(err)
	}
	if extended || filter.active() {
		st, err := readRepoState(ctx, git, line)
		if err != nil {
			return fail(err)
		}
		entry.state = &st
		if reason := filter.skipReason(st); reason != "" {
			t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
			return entry, false
		}
	}
	if extended {
		if entry.commitMsg, entry.commitDate, err = git.LastCommit(ctx, line); err != nil {
			return fail(fmt.Errorf("reading last commit: %w", err))
		}
	}
	t.record(opResult{
		project: project.Name, localPath: line, status: opOK, branch: entry.branch,
		state: entry.state, commitMsg: entry.commitMsg, commitDate: entry.commitDate,
	})
	return entry, true
}

func printBranchTable(entries []branchEntry, extended bool, t *tracker) {
	t.clearProgress() // erase the progress bar before the table prints to stdout
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if extended {
		fmt.Fprintln(w, "NAME\tBRANCH\tSTATUS\tDIRTY\tCOMMIT\tDATE\tPATH")
		for _, e := range entries {
			branch := e.branch
			if e.err != nil {
				branch = fmt.Sprintf("ERROR: %v", e.err)
			}
			status, dirty := "—", "—"
			if e.state != nil {
				status = t.colorSync(e.state.sync, false)
				dirty = cell(e.state.dirty.Symbols())
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", e.name, branch, status, dirty, cell(e.commitMsg), cell(e.commitDate), e.path)
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
	projects = filterByTag(projects, tagFilterValue(c))
	projects = filterDisabled(projects)
	t := newTracker(len(projects))
	git, err := core.NewGit(ui, core.WithSharedOutput(t.sharedOutput()))
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}
	extended := c.Bool("extended")
	filter := newGitStateFilter(c)
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
			if e, ok := branchProject(c, git, t, project, filter, extended); ok {
				entriesMu.Lock()
				entries = append(entries, e)
				entriesMu.Unlock()
			}
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("branch", warnings)
	} else {
		printBranchTable(entries, extended, t)
		t.printSummary(c.Bool("errors-last"), c.Bool("show-skipped"))
	}
	return buildError(t.errors())
}

func execProject(project gipProject, runner *core.CommandRunner, git *core.GitCommands, cmdName string, cmdArgs []string, filter gitStateFilter, t *tracker, c *cli.Context) {
	line, err := project.resolvedPath()
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
	reason, err := filter.check(ctx, git, line)
	if err != nil {
		t.record(opResult{project: project.Name, localPath: line, status: opError, err: err})
		return
	}
	if reason != "" {
		t.record(opResult{project: project.Name, localPath: line, status: opSkipped, reason: reason})
		return
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
	projects = filterByTag(projects, tagFilterValue(c))
	projects = filterDisabled(projects)
	t := newTracker(len(projects))
	runner := core.NewCommandRunner(ui, core.WithSharedOutputRunner(t.sharedOutput()))
	filter := newGitStateFilter(c)
	// git is needed (and must be installed) only to evaluate the filters.
	var git *core.GitCommands
	if filter.active() {
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
			execProject(project, runner, git, cmdName, cmdArgs, filter, t, c)
		}()
	}
	wg.Wait()
	if jsonMode {
		t.printJSON("exec", warnings)
	} else {
		t.printSummary(c.Bool("errors-last"), c.Bool("show-skipped"))
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
	uniqueInitNames(entries, scanDir)

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

// uniqueInitNames renames entries whose directory name is shared with another
// repository (e.g. work/a/api and work/b/api) to their path relative to root,
// since projectsList rejects duplicate project names.
func uniqueInitNames(entries []initEntry, root string) {
	count := make(map[string]int, len(entries))
	for _, e := range entries {
		count[e.Name]++
	}
	for i, e := range entries {
		if count[e.Name] < 2 {
			continue
		}
		if rel, err := filepath.Rel(root, e.LocalPath); err == nil {
			entries[i].Name = filepath.ToSlash(rel)
		} else {
			entries[i].Name = filepath.ToSlash(e.LocalPath)
		}
	}
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

// configurationFilePath returns the configuration file to use. When it was not
// given explicitly with -f, its path is printed on stderr: a .gip in the
// current directory takes precedence over ~/.gip, so the user must be able to
// see which file (and therefore which repositories and paths) is in effect.
func configurationFilePath(c *cli.Context) (string, error) {
	if c.String("f") != "" {
		return explicitConfigurationFilePath(c)
	}
	path, err := defaultConfigurationFilePath()
	if err == nil && !quietMode {
		fmt.Fprintf(os.Stderr, "gip: using configuration %s\n", path)
	}
	return path, err
}

func explicitConfigurationFilePath(c *cli.Context) (string, error) {
	abs, err := filepath.Abs(c.String("f"))
	if err != nil {
		return "", err
	}
	return filepath.FromSlash(filepath.Clean(abs)), nil
}
