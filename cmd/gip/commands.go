package main

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/enr/gip/lib/core"

	"github.com/urfave/cli/v2"
)

var commands = []*cli.Command{
	&commandStatus,
	&commandStatusFull,
	&commandList,
	&commandPull,
}

var commandStatus = cli.Command{
	Name:        "status",
	Aliases:     []string{"s"},
	Usage:       "",
	Description: `Prints modified files.`,
	Action:      doStatus,
}

var commandStatusFull = cli.Command{
	Name:        "statusfull",
	Aliases:     []string{"sf"},
	Usage:       "",
	Description: `Prints modified files and new ones.`,
	Action:      doStatusFull,
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
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "all", Aliases: []string{"a"}, Usage: `Pull all registered projects doing a checkout if needed. Otherwise only the projects already present are updated.`},
	},
}

func doStatus(c *cli.Context) error {
	return gitStatus(c, false)
}

func doStatusFull(c *cli.Context) error {
	return gitStatus(c, true)
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
	var line string
	errors := map[string]error{}
	for _, project := range projects {
		line, err = projectPath(project.LocalPath)
		if err != nil {
			// return exitErrorf(1, "Error loading project %s: %v", project.Name, err)
			errors[project.Name] = err
			continue
		}
		if isProjectDir(line) {
			err = git.Status(line, untracked)
			if err != nil {
				errors[project.Name] = err
				continue
			}
		} else {
			warnMissingDir(line)
		}
	}
	return buildError(errors)
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
			//return exitErrorf(1, "Error loading project %s: %v", project.Name, err)
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
	errors := map[string]error{}
	var line string
	for _, project := range projects {
		if project.pullNever() {
			ui.Confidentialf("Skip %s : pull policy never", project.Name)
			continue
		}
		line, err = projectPath(project.LocalPath)
		if err != nil {
			// return exitErrorf(1, "Error loading project %s: %v", project.Name, err)
			errors[project.Name] = err
			continue
		}
		if !isProjectDir(line) {
			warnMissingDir(line)
			if all || project.pullAlways() {
				err = git.Clone(project.Repository, line)
				if err != nil {
					errors[project.Name] = err
				}
			}
			continue
		}
		err = git.Pull(line)
		if err != nil {
			errors[project.Name] = err
			continue
		}
	}
	return buildError(errors)
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
	ui.Errorf(`Something gone wrong:`)
	return cli.NewExitError(message, exitCode)
}

func configurationFilePath(c *cli.Context) (string, error) {
	if c.String("f") != "" {
		abs, err := filepath.Abs(c.String("f"))
		if err != nil {
			return "", err
		}
		return filepath.FromSlash(filepath.Clean(abs)), nil
	}
	return defaultConfigurationFilePath(), nil
}
