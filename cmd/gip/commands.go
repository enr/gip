package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/enr/gip/lib/core"

	"github.com/urfave/cli/v2"
)

var commands = []*cli.Command{
	&commandStatus,
	&commandStatusFull,
	&commandList,
	&commandPull,
}

var parallelFlags = []cli.Flag{
	&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}, Value: 4, Usage: "maximum number of repos to operate on concurrently"},
	&cli.IntFlag{Name: "timeout", Aliases: []string{"t"}, Value: 0, Usage: "per-operation timeout in seconds (0 = no timeout)"},
}

var commandStatus = cli.Command{
	Name:        "status",
	Aliases:     []string{"s"},
	Usage:       "",
	Description: `Prints modified files.`,
	Action:      doStatus,
	Flags:       parallelFlags,
}

var commandStatusFull = cli.Command{
	Name:        "statusfull",
	Aliases:     []string{"sf"},
	Usage:       "",
	Description: `Prints modified files and new ones.`,
	Action:      doStatusFull,
	Flags:       parallelFlags,
}

var commandList = cli.Command{
	Name:        "list",
	Aliases:     []string{"ls"},
	Usage:       "",
	Description: `List projects`,
	Action:      doList,
}

var commandPull = cli.Command{
	Name:        "pull",
	Usage:       "",
	Description: `Pull projects`,
	Action:      doPull,
	Flags: append(parallelFlags,
		&cli.BoolFlag{Name: "all", Aliases: []string{"a"}, Usage: `Pull all registered projects doing a checkout if needed. Otherwise only the projects already present are updated.`},
	),
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
	git, err := core.NewGit(ui)
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}

	jobs := c.Int("jobs")
	if jobs < 1 {
		jobs = 1
	}

	var mu sync.Mutex
	errs := map[string]error{}
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
				mu.Lock()
				errs[project.Name] = err
				mu.Unlock()
				return
			}
			if isProjectDir(line) {
				ctx, cancel := opContext(c)
				defer cancel()
				err = git.Status(ctx, line, untracked)
				if err != nil {
					mu.Lock()
					errs[project.Name] = err
					mu.Unlock()
				}
			} else {
				warnMissingDir(line)
			}
		}()
	}
	wg.Wait()
	return buildError(errs)
}

func buildError(errors map[string]error) error {
	if len(errors) == 0 {
		return nil
	}
	var buffer bytes.Buffer
	for key, value := range errors {
		buffer.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
	}
	return exitError(1, buffer.String())
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
	var localPath string
	errors := map[string]error{}
	for _, project := range projects {
		localPath, err = projectPath(project.LocalPath)
		if err != nil {
			errors[project.Name] = err
			continue
		}
		if isProjectDir(localPath) {
			ui.Lifecyclef("- %s - %s (%s)", project.Name, localPath, project.repoProvider())
		} else {
			warnMissingDir(localPath)
		}
	}
	return buildError(errors)
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
	git, err := core.NewGit(ui)
	if err != nil {
		return exitErrorf(1, "Error loading git: %v", err)
	}

	jobs := c.Int("jobs")
	if jobs < 1 {
		jobs = 1
	}

	var mu sync.Mutex
	errs := map[string]error{}
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup

	for _, project := range projects {
		project := project
		if project.pullNever() {
			ui.Confidentialf("Skip %s : pull policy never", project.Name)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			line, err := projectPath(project.LocalPath)
			if err != nil {
				mu.Lock()
				errs[project.Name] = err
				mu.Unlock()
				return
			}
			if !isProjectDir(line) {
				warnMissingDir(line)
				if all || project.pullAlways() {
					ctx, cancel := opContext(c)
					defer cancel()
					err = git.Clone(ctx, project.Repository, line)
					if err != nil {
						mu.Lock()
						errs[project.Name] = err
						mu.Unlock()
					}
				}
				return
			}
			ctx, cancel := opContext(c)
			defer cancel()
			err = git.Pull(ctx, line)
			if err != nil {
				mu.Lock()
				errs[project.Name] = err
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return buildError(errs)
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
