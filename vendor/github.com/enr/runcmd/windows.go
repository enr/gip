// +build windows

package runcmd

func (c *Command) useShell() {
	command := c.CommandLine
	c.Exe = "cmd"
	c.Args = []string{"/C", command}
}
