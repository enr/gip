package main

/*
>go run main.go commands.go version.go utils.go statusfull
*/

import (
	"fmt"
	"os"

	"github.com/enr/clui"
	"github.com/urfave/cli"

	"github.com/enr/gip/lib/core"
)

var (
	ui              *clui.Clui
	versionTemplate = `%s
Revision: %s
Build date: %s
`
	appVersion = fmt.Sprintf(versionTemplate, core.Version, core.GitCommit, core.BuildTime)
)

func main() {
	app := cli.NewApp()
	app.Name = "gip"
	app.Version = appVersion
	app.Usage = "Keep tracks of your Git projects"
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "debug, d", Usage: "operates in debug mode: lot of output"},
		cli.BoolFlag{Name: "quiet, q", Usage: "operates in quiet mode"},
	}
	app.Author = ""
	app.Email = ""
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
		return nil
	}

	app.Commands = commands

	app.Run(os.Args)
}
