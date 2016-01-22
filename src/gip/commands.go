package main

import (
	"github.com/codegangsta/cli"
	"github.com/enr/clui"
)

var (
	ui *clui.Clui
)

func setup(c *cli.Context) error {
	if ui != nil {
		return nil
	}
	verbosityLevel := clui.VerbosityLevelMedium
	if c.Bool("debug") {
		verbosityLevel = clui.VerbosityLevelHigh
	}
	if c.Bool("quiet") {
		verbosityLevel = clui.VerbosityLevelLow
	}
	ui, _ = clui.NewClui(func(ui *clui.Clui) {
		ui.VerbosityLevel = verbosityLevel
	})
	return nil
}

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
	Before:      setup,
}

var commandStatusFull = cli.Command{
	Name:        "statusfull",
	ShortName:   "sf",
	Usage:       "",
	Description: `Prints modified files and new ones.`,
	Action:      doStatusFull,
	Before:      setup,
}

var commandList = cli.Command{
	Name:        "list",
	ShortName:   "ls",
	Usage:       "",
	Description: `List projects registered in ~/.gip`,
	Action:      doList,
	Before:      setup,
}

var commandPull = cli.Command{
	Name:        "pull",
	ShortName:   "",
	Usage:       "",
	Description: `Pull projects`,
	Action:      doPull,
	Before:      setup,
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "all, a", Usage: `Pull all registered projects doing a checkout if needed. Otherwise only the projects already present are updated.`},
	},
}

func doStatus(c *cli.Context) {
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
	//var name string
	var localPath string
	projects, _ := projectsList(configurationFile)
	for _, project := range projects {
		//name = project.Name
		localPath, _ = projectPath(project.LocalPath)
		if isProjectDir(localPath) {
			ui.Lifecyclef("%s", localPath)
		} else {
			ui.Warnf("%s (not a project dir)", localPath)
		}
	}
}

func doPull(c *cli.Context) {
	ui.Warnf("PULL COMMAND NOT YET IMPLEMENTED")
	configurationFile := configurationFilePath()
	ui.Warnf("%s PULL all? %t", configurationFile, c.Bool("all"))
	var line string
	projects, _ := projectsList(configurationFile)
	for _, project := range projects {
		line, _ = projectPath(project.LocalPath)
		if isProjectDir(line) {
			ui.Lifecyclef("%s git pull %s", project.Name, project.Repository)
		} else {
			ui.Warnf("%s is not a project dir", line)
		}
	}
}
