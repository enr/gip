package main

/*
>go run main.go commands.go version.go utils.go statusfull
*/

import (
	"os"

	"github.com/codegangsta/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "gip"
	app.Version = Version
	app.Usage = "Keep tracks of your Git projects"
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "debug, d", Usage: "operates in debug mode: lot of output"},
		cli.BoolFlag{Name: "quiet, q", Usage: "operates in quiet mode"},
	}
	app.Author = ""
	app.Email = ""
	app.EnableBashCompletion = true

	app.Commands = Commands

	app.Run(os.Args)
}
