package main

import (
	"bytes"
	"context"
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

var parallelFlags = []cli.Flag{
	&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}, Value: 4, Usage: "maximum number of repos to operate on concurrently"},
	&cli.IntFlag{Name: "timeout", Aliases: []string{"t"}, Value: 0, Usage: "per-operation timeout in seconds (0 = no timeout)"},
	tagFlag,
	errorsLastFlag,
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
	Flags:       []cli.Flag{tagFlag},
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
	Flags:       parallelFlags,
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
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "output file path (default: ~/.gip)"},
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
	projects, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	git, err := core.NewGit(ui)
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}

	jobs := c.Int("jobs")
	if jobs < 1 {
		jobs = 1
	}

	t := newTracker(len(projects))
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup

	untrackedFlag := "--untracked-files=no"
	if untracked {
		untrackedFlag = "--untracked-files"
	}

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
				warnMissingDir(line)
				t.record(opResult{project: project.Name, status: opSkipped, reason: "not a project dir"})
				return
			}
			if noopMode {
				t.printNoop("%s → git status --porcelain %s  (in %s)", project.Name, untrackedFlag, line)
				t.record(opResult{project: project.Name, status: opOK})
				return
			}
			ctx, cancel := opContext(c)
			defer cancel()
			if err = git.Status(ctx, line, untracked); err != nil {
				t.record(opResult{project: project.Name, status: opError, err: err})
			} else {
				t.record(opResult{project: project.Name, status: opOK})
			}
		}()
	}
	wg.Wait()
	t.printSummary(c.Bool("errors-last"))
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
	pathErr                            error
}

func doList(c *cli.Context) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))

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

		localPath, err := projectPath(project.LocalPath)
		if err != nil {
			errs[project.Name] = err
			rows = append(rows, listRow{
				name: project.Name, path: project.LocalPath + " (ERROR)",
				policy: policy, provider: provider, tags: tags, pathErr: err,
			})
			continue
		}
		if !isProjectDir(localPath) {
			localPath += " (missing)"
		}
		rows = append(rows, listRow{
			name: project.Name, path: localPath,
			policy: policy, provider: provider, tags: tags,
		})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if hasTags {
		fmt.Fprintln(w, "NAME\tPATH\tPOLICY\tPROVIDER\tTAGS")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.name, r.path, r.policy, r.provider, r.tags)
		}
	} else {
		fmt.Fprintln(w, "NAME\tPATH\tPOLICY\tPROVIDER")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.name, r.path, r.policy, r.provider)
		}
	}
	w.Flush()
	return buildError(errs)
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
	projects, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	git, err := core.NewGit(ui)
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}

	jobs := c.Int("jobs")
	if jobs < 1 {
		jobs = 1
	}

	t := newTracker(len(projects))
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
					t.printNoop("%s → SALTATO  (pull_policy: never)", project.Name)
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
				warnMissingDir(line)
				if all || project.pullAlways() {
					if noopMode {
						t.printNoop("%s → git clone %s %s", project.Name, project.Repository, line)
						t.record(opResult{project: project.Name, status: opOK})
						return
					}
					ctx, cancel := opContext(c)
					defer cancel()
					if err = git.Clone(ctx, project.Repository, line); err != nil {
						t.record(opResult{project: project.Name, status: opError, err: err})
					} else {
						t.record(opResult{project: project.Name, status: opOK})
					}
				} else {
					if noopMode {
						t.printNoop("%s → SALTATO  (directory mancante)", project.Name)
					}
					t.record(opResult{project: project.Name, status: opSkipped, reason: "local dir missing"})
				}
				return
			}
			if noopMode {
				t.printNoop("%s → git pull  (in %s)", project.Name, line)
				t.record(opResult{project: project.Name, status: opOK})
				return
			}
			ctx, cancel := opContext(c)
			defer cancel()
			if err = git.Pull(ctx, line); err != nil {
				t.record(opResult{project: project.Name, status: opError, err: err})
			} else {
				t.record(opResult{project: project.Name, status: opOK})
			}
		}()
	}
	wg.Wait()
	t.printSummary(c.Bool("errors-last"))
	return buildError(t.errors())
}

func doFetch(c *cli.Context) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	git, err := core.NewGit(ui)
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}

	jobs := c.Int("jobs")
	if jobs < 1 {
		jobs = 1
	}

	t := newTracker(len(projects))
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
					t.printNoop("%s → SALTATO  (pull_policy: never)", project.Name)
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
				warnMissingDir(line)
				if noopMode {
					t.printNoop("%s → SALTATO  (directory mancante)", project.Name)
				}
				t.record(opResult{project: project.Name, status: opSkipped, reason: "local dir missing"})
				return
			}
			if noopMode {
				t.printNoop("%s → git fetch --all --prune  (in %s)", project.Name, line)
				t.record(opResult{project: project.Name, status: opOK})
				return
			}
			ctx, cancel := opContext(c)
			defer cancel()
			if err = git.Fetch(ctx, line); err != nil {
				t.record(opResult{project: project.Name, status: opError, err: err})
			} else {
				t.record(opResult{project: project.Name, status: opOK})
			}
		}()
	}
	wg.Wait()
	t.printSummary(c.Bool("errors-last"))
	return buildError(t.errors())
}

type branchEntry struct {
	name   string
	path   string
	branch string
	err    error
}

func doBranch(c *cli.Context) error {
	configurationFile, err := configurationFilePath(c)
	if err != nil {
		return exitErrorf(1, "Error loading configuration file %s: %v", c.String("f"), err)
	}
	projects, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	git, err := core.NewGit(ui)
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}

	jobs := c.Int("jobs")
	if jobs < 1 {
		jobs = 1
	}

	t := newTracker(len(projects))
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

			entry := branchEntry{name: project.Name}
			line, err := projectPath(project.LocalPath)
			if err != nil {
				entry.err = err
				t.record(opResult{project: project.Name, status: opError, err: err})
				entriesMu.Lock()
				entries = append(entries, entry)
				entriesMu.Unlock()
				return
			}
			entry.path = line
			if !isProjectDir(line) {
				entry.branch = "(missing)"
				if noopMode {
					t.printNoop("%s → SALTATO  (directory mancante)", project.Name)
				}
				t.record(opResult{project: project.Name, status: opSkipped, reason: "local dir missing"})
			} else if noopMode {
				t.printNoop("%s → git rev-parse --abbrev-ref HEAD  (in %s)", project.Name, line)
				entry.branch = "(noop)"
				t.record(opResult{project: project.Name, status: opOK})
			} else {
				ctx, cancel := opContext(c)
				defer cancel()
				entry.branch, entry.err = git.CurrentBranch(ctx, line)
				if entry.err != nil {
					t.record(opResult{project: project.Name, status: opError, err: entry.err})
				} else {
					t.record(opResult{project: project.Name, status: opOK})
				}
			}
			entriesMu.Lock()
			entries = append(entries, entry)
			entriesMu.Unlock()
		}()
	}
	wg.Wait()

	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tBRANCH\tPATH")
	for _, e := range entries {
		branch := e.branch
		if e.err != nil {
			branch = fmt.Sprintf("ERROR: %v", e.err)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", e.name, branch, e.path)
	}
	w.Flush()

	t.printSummary(c.Bool("errors-last"))
	return buildError(t.errors())
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
	projects, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	projects = filterByTag(projects, c.String("tag"))
	runner := core.NewCommandRunner(ui)

	jobs := c.Int("jobs")
	if jobs < 1 {
		jobs = 1
	}

	t := newTracker(len(projects))
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
				warnMissingDir(line)
				if noopMode {
					t.printNoop("%s → SALTATO  (directory mancante)", project.Name)
				}
				t.record(opResult{project: project.Name, status: opSkipped, reason: "not a project dir"})
				return
			}
			if noopMode {
				t.printNoop("%s → %s %s  (in %s)", project.Name, cmdName, strings.Join(cmdArgs, " "), line)
				t.record(opResult{project: project.Name, status: opOK})
				return
			}
			ctx, cancel := opContext(c)
			defer cancel()
			if err = runner.Run(ctx, line, cmdName, cmdArgs); err != nil {
				t.record(opResult{project: project.Name, status: opError, err: err})
			} else {
				t.record(opResult{project: project.Name, status: opOK})
			}
		}()
	}
	wg.Wait()
	t.printSummary(c.Bool("errors-last"))
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
		home, err := os.UserHomeDir()
		if err != nil {
			return exitErrorf(1, "Error retrieving home directory: %v", err)
		}
		outputPath = filepath.Join(home, ".gip")
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
