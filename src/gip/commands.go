package main

import (
	"fmt"
	"github.com/urfave/cli"
)

var Commands = []cli.Command{
	commandStatus,
	commandStatusFull,
	commandList,
	commandPull,
}

var commandStatus = cli.Command{
	Name:        "status",
	ShortName:   "s",
	Usage:       "",
	Description: `Prints modified files.`,
	Action:      doStatus,
}

var commandStatusFull = cli.Command{
	Name:        "statusfull",
	ShortName:   "sf",
	Usage:       "",
	Description: `Prints modified files and new ones.`,
	Action:      doStatusFull,
}

var commandList = cli.Command{
	Name:        "list",
	ShortName:   "ls",
	Usage:       "",
	Description: `List projects`,
	Action:      doList,
}

var commandPull = cli.Command{
	Name:        "pull",
	ShortName:   "",
	Usage:       "",
	Description: `Pull projects`,
	Action:      doPull,
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "all, a", Usage: `Pull all registered projects doing a checkout if needed. Otherwise only the projects already present are updated.`},
	},
}

func doStatus(c *cli.Context) {
	fmt.Printf("--- %v \n", c.Bool("debug"))
	gitStatus(false)
}

func doStatusFull(c *cli.Context) {
	gitStatus(true)
}

func gitStatus(untracked bool) {
	configurationFile := configurationFilePath()
	var line string
	projects, _ := projectsList(configurationFile)
	for _, project := range projects {
		line, _ = projectPath(project.LocalPath)
		if isProjectDir(line) {
			executeGitStatus(line, untracked)
		} else {
			ui.Warnf("%s is not a project dir", line)
		}
	}
}

func doList(c *cli.Context) {
	configurationFile := configurationFilePath()
	var localPath string
	projects, _ := projectsList(configurationFile)
	for _, project := range projects {
		localPath, _ = projectPath(project.LocalPath)
		if isProjectDir(localPath) {
			ui.Lifecyclef("%s", localPath)
		} else {
			ui.Warnf("%s (not a project dir)", localPath)
		}
	}
}

func doPull(c *cli.Context) {
	configurationFile := configurationFilePath()
	all := c.Bool("all")
	ui.Confidentialf("%s PULL all? %t", configurationFile, all)
	var line string
	projects, _ := projectsList(configurationFile)
	for _, project := range projects {
		line, _ = projectPath(project.LocalPath)
		if isProjectDir(line) {
			executeGitPull(line)
		} else {
			ui.Confidentialf("%s not a project dir", line)
			if all {
				executeGitClone(project.Repository, line)
				executeGitPull(line)
			} else {
				ui.Warnf("%s (not a project dir)", line)
			}
		}
	}
}
