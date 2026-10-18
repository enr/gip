package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/enr/gip/lib/core"
)

// gip check: read-only verification of the configuration against the disk
// (and, with --remote, against the remote repositories). Errors make the
// command fail; warnings are reported only.

var checkFlags = []cli.Flag{
	tagFlag,
	&cli.BoolFlag{Name: "remote", Aliases: []string{"r"}, Usage: `also check that each repository is reachable ("git ls-remote"; needs network access)`},
	&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}, Value: 4, Usage: fmt.Sprintf("maximum number of repos to check concurrently (1-%d)", maxJobs)},
	&cli.IntFlag{Name: "timeout", Aliases: []string{"t"}, Value: 0, Usage: "per-repo timeout in seconds (0 = no timeout)"},
}

var commandCheck = cli.Command{
	Name:  "check",
	Usage: "check the configuration for wrong paths, mismatched remotes and missing branches",
	Description: `Verifies every enabled project without changing anything:
local_path exists and is a git repository, one of its remotes points to
"repository", no two projects share a directory, the checked-out branch and
the extra "branches" exist and track an upstream.
With --remote the repositories are also contacted ("git ls-remote").
Exits with status 1 when an error is found; warnings do not change it.`,
	Action:       doCheck,
	Flags:        checkFlags,
	BashComplete: bashCompleteWithFlags(checkFlags),
}

type checkSeverity string

const (
	checkError   checkSeverity = "error"
	checkWarning checkSeverity = "warning"
)

type checkIssue struct {
	Severity checkSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// checkEntry is the outcome of one project; index keeps the configuration order.
type checkEntry struct {
	index  int
	name   string
	path   string
	issues []checkIssue
}

func (e *checkEntry) errorf(format string, args ...interface{}) {
	e.issues = append(e.issues, checkIssue{checkError, fmt.Sprintf(format, args...)})
}

func (e *checkEntry) warnf(format string, args ...interface{}) {
	e.issues = append(e.issues, checkIssue{checkWarning, fmt.Sprintf(format, args...)})
}

func (e checkEntry) has(s checkSeverity) bool {
	for _, i := range e.issues {
		if i.Severity == s {
			return true
		}
	}
	return false
}

func (e checkEntry) status() string {
	switch {
	case e.has(checkError):
		return "error"
	case e.has(checkWarning):
		return "warning"
	}
	return "ok"
}

func doCheck(c *cli.Context) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, warnings, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterDisabled(projects)
	// Shared directories are looked for among all the enabled projects, not
	// only among the ones selected by --tag.
	sharedPaths := projectsByPath(projects)
	projects = filterByTag(projects, tagFilterValue(c))
	jobs, err := resolveJobs(c)
	if err != nil {
		return exitErrorf(1, "%v", err)
	}
	t := newTracker(len(projects))
	git, err := core.NewGit(ui, core.WithSharedOutput(t.sharedOutput()))
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}
	remote := c.Bool("remote")

	var entriesMu sync.Mutex
	entries := make([]checkEntry, 0, len(projects))
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for i, project := range projects {
		i, project := i, project
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			e := checkProject(c, git, t, project, remote)
			e.index = i
			if others := sharedPaths[e.path]; e.path != "" && len(others) > 1 {
				e.errorf("local_path shared with %s", strings.Join(without(others, project.Name), ", "))
			}
			entriesMu.Lock()
			entries = append(entries, e)
			entriesMu.Unlock()
			status := opOK
			if e.has(checkError) {
				status = opError
			}
			t.record(opResult{project: project.Name, localPath: e.path, status: status})
		}()
	}
	wg.Wait()
	sort.Slice(entries, func(a, b int) bool { return entries[a].index < entries[b].index })

	if jsonMode {
		printCheckJSON(entries, warnings, t)
	} else {
		printCheckReport(entries, warnings, t)
	}
	errs := make(map[string]error)
	for _, e := range entries {
		for _, i := range e.issues {
			if i.Severity == checkError {
				errs[e.name] = fmt.Errorf("%s", i.Message)
				break
			}
		}
	}
	return buildError(errs)
}

// checkProject runs the per-project checks. It never modifies the repository.
func checkProject(c *cli.Context, git *core.GitCommands, t *tracker, project gipProject, remote bool) checkEntry {
	e := checkEntry{name: project.Name}
	line, err := project.resolvedPath()
	if err != nil {
		e.errorf("local_path %q: %v", project.LocalPath, err)
		return e
	}
	e.path = filepath.Clean(line)
	if noopMode {
		what := "check"
		if remote {
			what += " + git ls-remote"
		}
		t.printNoop("%s → %s  (in %s)", project.Name, what, e.path)
		return e
	}
	ctx, cancel := opContext(c)
	defer cancel()

	if remote {
		checkReachable(ctx, git, project, &e)
	}
	if !checkLocalDir(&e) {
		return e
	}
	checkRemotes(ctx, git, project, &e)
	checkBranches(ctx, git, project, &e)
	return e
}

// checkReachable contacts project.Repository with "git ls-remote".
func checkReachable(ctx context.Context, git *core.GitCommands, project gipProject, e *checkEntry) {
	if project.Repository == "" {
		return
	}
	dir := ""
	if isProjectDir(e.path) {
		dir = e.path
	}
	err := git.LsRemote(ctx, dir, project.Repository)
	switch {
	case err == nil:
	case isAuthError(err):
		// Hosts answer a missing private repository with an authentication
		// request too: existence cannot be told.
		e.warnf("repository %s not verified, authentication required: %v", project.Repository, err)
	default:
		e.errorf("repository %s not reachable: %v", project.Repository, err)
	}
}

// checkLocalDir reports whether local_path is a git repository, the
// precondition of every other local check.
func checkLocalDir(e *checkEntry) bool {
	info, err := os.Stat(e.path)
	switch {
	case os.IsNotExist(err):
		e.warnf("directory missing (gip pull -a clones it)")
	case err != nil:
		e.errorf("cannot read local_path: %v", err)
	case !info.IsDir():
		e.errorf("local_path is not a directory")
	case !isProjectDir(e.path):
		e.errorf("local_path is not a git repository")
	default:
		return true
	}
	return false
}

// checkBranches verifies that the checked-out branch and the extra branches
// exist and track an upstream.
func checkBranches(ctx context.Context, git *core.GitCommands, project gipProject, e *checkEntry) {
	st, err := readRepoState(ctx, git, e.path)
	if err != nil {
		e.errorf("%v", err)
		return
	}
	switch {
	case st.sync.Branch == "(detached)":
		e.warnf("detached HEAD")
	case st.sync.NoRemote:
		e.warnf("branch %s has no upstream", st.sync.Branch)
	}

	extras := project.extraBranches(st.sync.Branch)
	if len(extras) == 0 {
		return
	}
	infos, err := git.BranchInfos(ctx, e.path, extras)
	if err != nil {
		e.errorf("reading branches: %v", err)
		return
	}
	for _, b := range infos {
		switch {
		case !b.Exists:
			e.warnf("branch %s (in branches) not found locally", b.Name)
		case b.NoRemote:
			e.warnf("branch %s (in branches) has no upstream", b.Name)
		}
	}
}

// isAuthError reports whether git failed for lack of credentials rather than
// because the repository does not exist or the host is unreachable.
func isAuthError(err error) bool {
	msg := err.Error()
	for _, m := range []string{"could not read Username", "could not read Password", "terminal prompts disabled", "Authentication failed", "Permission denied (publickey"} {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

// checkRemotes verifies that one of the remotes points to project.Repository.
func checkRemotes(ctx context.Context, git *core.GitCommands, project gipProject, e *checkEntry) {
	if project.Repository == "" {
		return
	}
	urls, err := git.RemoteURLs(ctx, e.path)
	if err != nil {
		e.errorf("reading remotes: %v", err)
		return
	}
	if len(urls) == 0 {
		e.errorf("no remote configured (repository is %s)", project.Repository)
		return
	}
	names := make([]string, 0, len(urls))
	for name, u := range urls {
		if sameRepository(u, project.Repository) {
			return
		}
		names = append(names, name)
	}
	sort.Strings(names)
	found := make([]string, 0, len(names))
	for _, name := range names {
		found = append(found, name+" "+urls[name])
	}
	e.errorf("no remote points to repository %s (found: %s)", project.Repository, strings.Join(found, ", "))
}

// sameRepository reports whether two repository URLs name the same
// repository: protocol, user, port, a trailing ".git" or "/" and the case of
// host and path are ignored, so https://host/a/b and git@host:a/b.git match.
func sameRepository(a, b string) bool {
	return repositoryKey(a) == repositoryKey(b)
}

func repositoryKey(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")
	var host, path string
	switch {
	case strings.Contains(s, "://"):
		u, err := url.Parse(s)
		if err != nil {
			return s
		}
		if u.Scheme == "file" {
			return filepath.Clean(u.Path)
		}
		host, path = u.Hostname(), u.Path
	case isSCPLike(s):
		hostPart, p, _ := strings.Cut(s, ":")
		if at := strings.LastIndex(hostPart, "@"); at >= 0 {
			hostPart = hostPart[at+1:]
		}
		host, path = hostPart, p
	default:
		// Local path: compared as is, case included.
		return filepath.Clean(s)
	}
	return strings.ToLower(host + "/" + strings.Trim(path, "/"))
}

// isSCPLike matches git's "[user@]host:path" form, which has a colon before
// any slash (a Windows drive letter such as C:\x is excluded).
func isSCPLike(s string) bool {
	colon := strings.Index(s, ":")
	if colon <= 1 {
		return false
	}
	slash := strings.IndexAny(s, `/\`)
	return slash < 0 || colon < slash
}

// projectsByPath maps each resolved local_path to the projects using it.
func projectsByPath(projects []gipProject) map[string][]string {
	m := make(map[string][]string)
	for _, p := range projects {
		line, err := p.resolvedPath()
		if err != nil {
			continue
		}
		key := filepath.Clean(line)
		m[key] = append(m[key], p.Name)
	}
	return m
}

func without(names []string, name string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n != name {
			out = append(out, n)
		}
	}
	return out
}

// printCheckReport lists the projects with issues, then a summary line.
func printCheckReport(entries []checkEntry, warnings []string, t *tracker) {
	t.clearProgress()
	var okCount, errCount, warnCount int
	for _, e := range entries {
		switch e.status() {
		case "ok":
			okCount++
		case "error":
			errCount++
		default:
			warnCount++
		}
		if quietMode || len(e.issues) == 0 {
			continue
		}
		fmt.Fprintf(os.Stdout, "%s  %s\n", e.name, t.applyStyle(styleDivider, cell(e.path)))
		for _, i := range e.issues {
			mark := t.applyStyle(styleSkip, "[WARN]")
			if i.Severity == checkError {
				mark = t.applyStyle(styleErr, "[ERR] ")
			}
			fmt.Fprintf(os.Stdout, "  %s %s\n", mark, i.Message)
		}
	}
	okStr := t.styleIf(okCount > 0, styleOK, fmt.Sprintf("OK: %d", okCount))
	errStr := t.styleIf(errCount > 0, styleErr, fmt.Sprintf("Errors: %d", errCount))
	warnStr := t.styleIf(warnCount > 0, styleSkip, fmt.Sprintf("Warnings: %d", warnCount))
	fmt.Fprintf(os.Stdout, "%s\n", t.applyStyle(styleDivider, "─────────────────────────────────────────"))
	fmt.Fprintf(os.Stdout, "%s   %s   %s   Duration: %.1fs\n", okStr, errStr, warnStr, time.Since(t.started).Seconds())
	if len(warnings) > 0 {
		fmt.Fprintf(os.Stdout, "Configuration warnings: %d (printed above)\n", len(warnings))
	}
}

type checkProjectJSON struct {
	Name      string       `json:"name"`
	LocalPath string       `json:"local_path"`
	Status    string       `json:"status"`
	Issues    []checkIssue `json:"issues"`
}

type checkSummaryJSON struct {
	Total      int   `json:"total"`
	OK         int   `json:"ok"`
	Errors     int   `json:"errors"`
	Warnings   int   `json:"warnings"`
	DurationMs int64 `json:"duration_ms"`
}

type checkOutputJSON struct {
	Command   string             `json:"command"`
	Timestamp string             `json:"timestamp"`
	Projects  []checkProjectJSON `json:"projects"`
	Summary   checkSummaryJSON   `json:"summary"`
	Warnings  []string           `json:"warnings"`
}

func printCheckJSON(entries []checkEntry, warnings []string, t *tracker) {
	t.clearProgress()
	out := checkOutputJSON{
		Command:   "check",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Projects:  make([]checkProjectJSON, 0, len(entries)),
		Warnings:  warnings,
	}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	for _, e := range entries {
		p := checkProjectJSON{Name: e.name, LocalPath: e.path, Status: e.status(), Issues: e.issues}
		if p.Issues == nil {
			p.Issues = []checkIssue{}
		}
		switch p.Status {
		case "ok":
			out.Summary.OK++
		case "error":
			out.Summary.Errors++
		default:
			out.Summary.Warnings++
		}
		out.Projects = append(out.Projects, p)
	}
	out.Summary.Total = len(entries)
	out.Summary.DurationMs = time.Since(t.started).Milliseconds()
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintf(os.Stdout, "%s\n", data)
}
