package main

/*
>go run main.go commands.go version.go utils.go statusfull
*/

import (
	"fmt"
	"os"

	"github.com/enr/clui"
	"github.com/urfave/cli/v2"

	"github.com/enr/gip/lib/core"
)

var (
	ui              *clui.Clui
	versionTemplate = `%s
Revision: %s
Build date: %s
`
	appVersion        = fmt.Sprintf(versionTemplate, core.Version, core.GitCommit, core.BuildTime)
	ignoreMissingDirs bool
)

func main() {
	app := cli.NewApp()
	app.Name = "gip"
	app.Version = appVersion
	app.Usage = "Keep tracks of your Git projects"
	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "path to the configuration file to use (if not set will be picked ${HOME}/.gip)"},
		&cli.BoolFlag{Name: "debug", Aliases: []string{"d"}, Usage: "operates in debug mode: lot of output"},
		&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "operates in quiet mode"},
		&cli.BoolFlag{Name: "ignore-missing", Aliases: []string{"m"}, Usage: "ignores missing local directories, otherwise prints a warn"},
	}
	app.EnableBashCompletion = true

	app.Before = func(c *cli.Context) error {
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
		ignoreMissingDirs = c.Bool("m")
		return nil
	}

	app.Commands = commands

	app.Run(os.Args)
}
