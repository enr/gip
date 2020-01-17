package main

import (
	"fmt"

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
	return gitStatus(false)
}

func doStatusFull(c *cli.Context) error {
	return gitStatus(true)
}

func gitStatus(untracked bool) error {
	configurationFile := configurationFilePath()
	var line string
	projects, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	for _, project := range projects {
		line, err = projectPath(project.LocalPath)
		if err != nil {
			return exitErrorf(1, "Error loading project %s: %v", project.Name, err)
		}
		if isProjectDir(line) {
			executeGitStatus(line, untracked)
		} else if !ignoreMissingDirs {
			ui.Warnf("%s is not a project directory", line)
		}
	}
	return nil
}

func doList(c *cli.Context) error {
	configurationFile := configurationFilePath()
	var localPath string
	projects, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	for _, project := range projects {
		localPath, err = projectPath(project.LocalPath)
		if err != nil {
			return exitErrorf(1, "Error loading project %s: %v", project.Name, err)
		}
		if isProjectDir(localPath) {
			ui.Lifecyclef("- %s - %s (%s)", project.Name, localPath, project.repoProvider())
		} else if !ignoreMissingDirs {
			ui.Warnf("- missing %s - %s (%s)", project.Name, localPath, project.repoProvider())
		}
	}
	return nil
}

func doPull(c *cli.Context) error {
	configurationFile := configurationFilePath()
	args := c.Args().Slice()
	if len(args) > 0 {
		return exitErrorf(1, "Pull command does not accept any argument, found: %v", args)
	}
	all := c.Bool("all")
	ui.Confidentialf("%s PULL all? %t", configurationFile, all)
	var line string
	projects, err := projectsList(configurationFile)
	if err != nil {
		return exitErrorf(1, "Error loading projects list: %v", err)
	}
	for _, project := range projects {
		if project.pullNever() {
			ui.Confidentialf("Skip %s : pull policy never", project.Name)
			continue
		}
		line, err = projectPath(project.LocalPath)
		if err != nil {
			return exitErrorf(1, "Error loading project %s: %v", project.Name, err)
		}
		if isProjectDir(line) {
			executeGitPull(line)
		} else {
			ui.Confidentialf("%s not a project dir", line)
			if all || project.pullAlways() {
				executeGitClone(project.Repository, line)
			} else if !ignoreMissingDirs {
				ui.Warnf("%s (not a project dir)", line)
			}
		}
	}
	return nil
}

func exitErrorf(exitCode int, template string, args ...interface{}) error {
	ui.Errorf(`Something gone wrong:`)
	return cli.NewExitError(fmt.Sprintf(template, args...), exitCode)
}
