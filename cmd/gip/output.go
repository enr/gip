package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

// colorEnabled reports whether ANSI colors may be written to f.
//
//   - NO_COLOR set to any non-empty value disables colors (https://no-color.org).
//   - FORCE_COLOR set to a non-empty value other than "0" enables them even when
//     f is not a terminal (e.g. CI logs that render ANSI).
//   - Otherwise colors are used only when f is a terminal and TERM is not "dumb".
func colorEnabled(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if v := os.Getenv("FORCE_COLOR"); v != "" && v != "0" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTTY(f)
}

// ansiStripWriter removes ANSI color sequences before writing to w. It wraps
// the clui writers when the stream does not accept colors: clui writes each
// message with a single Write, so escape sequences are never split.
type ansiStripWriter struct {
	w io.Writer
}

func (s ansiStripWriter) Write(p []byte) (int, error) {
	if _, err := s.w.Write(ansiRe.ReplaceAll(p, nil)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// plainWriterUnlessColor returns w unchanged when f accepts colors, otherwise a
// writer stripping ANSI sequences.
func plainWriterUnlessColor(w io.Writer, f *os.File) io.Writer {
	if colorEnabled(f) {
		return w
	}
	return ansiStripWriter{w: w}
}

// hoistGlobalFlags moves the global flags written after the command name
// before it, so "gip status --json" behaves like "gip --json status".
//
// Only the arguments between the command and the first positional argument
// (or "--") are looked at: what follows belongs to the command, e.g. the
// command run by "gip exec -- git log -q". Values of command flags ("-j 8")
// are skipped, so they are never mistaken for positional arguments.
func hoistGlobalFlags(args []string, globals []cli.Flag, commands []*cli.Command) []string {
	globalValue := flagArity(globals)
	cmdIdx := commandIndex(args, globalValue)
	if cmdIdx < 0 {
		return args
	}
	cmd := findCommand(commands, args[cmdIdx])
	if cmd == nil {
		return args
	}
	localValue := flagArity(cmd.Flags)

	var hoisted, rest []string
	i := cmdIdx + 1
	for i < len(args) && args[i] != "--" && isFlagArg(args[i]) {
		name := flagName(args[i])
		takes, local := localValue[name]
		gtakes, global := globalValue[name]
		var n int
		switch {
		case local: // command flags win over global ones with the same name
			n = flagSpan(args, i, takes)
			rest = append(rest, args[i:i+n]...)
		case global:
			n = flagSpan(args, i, gtakes)
			hoisted = append(hoisted, args[i:i+n]...)
		default: // unknown flag: left in place for the parser to report
			n = 1
			rest = append(rest, args[i])
		}
		i += n
	}
	if len(hoisted) == 0 {
		return args
	}
	out := make([]string, 0, len(args))
	out = append(out, args[:cmdIdx]...)
	out = append(out, hoisted...)
	out = append(out, args[cmdIdx])
	out = append(out, rest...)
	return append(out, args[i:]...)
}

// commandIndex returns the position of the command name in args: the first
// positional argument after the global flags (and their values), or -1.
func commandIndex(args []string, globalValue map[string]bool) int {
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return -1
		}
		if !isFlagArg(a) {
			return i
		}
		if globalValue[flagName(a)] {
			i += flagSpan(args, i, true) - 1 // skip the value of "-f path"
		}
	}
	return -1
}

// flagSpan is the number of arguments taken by the flag at args[i]: 2 for a
// flag with a separate value ("-j 8"), 1 otherwise ("-q", "--jobs=8").
func flagSpan(args []string, i int, takesValue bool) int {
	if takesValue && !strings.Contains(args[i], "=") && i+1 < len(args) {
		return 2
	}
	return 1
}

func isFlagArg(a string) bool {
	return len(a) > 1 && strings.HasPrefix(a, "-")
}

// flagName returns the name of a flag argument without dashes and value:
// "--file=x" → "file", "-q" → "q".
func flagName(a string) string {
	n := strings.TrimLeft(a, "-")
	if i := strings.Index(n, "="); i >= 0 {
		n = n[:i]
	}
	return n
}

// flagArity maps every name and alias of flags to whether it takes a value.
func flagArity(flags []cli.Flag) map[string]bool {
	m := make(map[string]bool)
	for _, f := range flags {
		takes := false
		if v, ok := f.(cli.DocGenerationFlag); ok {
			takes = v.TakesValue()
		}
		for _, n := range f.Names() {
			m[n] = takes
		}
	}
	return m
}

func findCommand(commands []*cli.Command, name string) *cli.Command {
	for _, c := range commands {
		if c.HasName(name) {
			return c
		}
	}
	return nil
}

// commandNotFound reports an unknown command with a suggestion and exits with
// status 2 (usage error) instead of falling back to the help topic lookup.
func commandNotFound(c *cli.Context, name string) {
	msg := fmt.Sprintf("gip: unknown command %q.", name)
	if cli.SuggestCommand != nil {
		if s := cli.SuggestCommand(c.App.Commands, name); s != "" {
			msg += " " + s
		}
	}
	msg += " Run 'gip --help' for the list of commands."
	cli.HandleExitCoder(cli.Exit(msg, 2))
}

// errorSummaryLine condenses an operation error to one line for the final
// error list: the detailed output of git was already printed inline, so only
// its most telling line is kept ("fatal: ...", "error: ...", else the last
// non-empty one), followed by a hint when the cause is a known one.
func errorSummaryLine(err error) string {
	if err == nil {
		return ""
	}
	full := err.Error()
	lines := strings.Split(strings.TrimSpace(full), "\n")
	head := strings.TrimSpace(lines[0])
	line := head
	if len(lines) > 1 {
		pick := ""
		for _, l := range lines[1:] {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "fatal:") || strings.HasPrefix(l, "error:") {
				pick = l
				break
			}
			if l != "" {
				pick = l
			}
		}
		if pick != "" {
			// "exit status 128: Cloning into ..." → keep the exit status prefix.
			if i := strings.Index(head, ": "); i >= 0 {
				head = head[:i]
			}
			line = head + ": " + pick
		}
	}
	if hint := errorHint(full); hint != "" {
		line += " (hint: " + hint + ")"
	}
	return line
}

// errorHint suggests the next step for well-known failures.
func errorHint(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "could not read username") || strings.Contains(m, "terminal prompts disabled"):
		return "authentication required: configure a git credential helper or use an SSH URL"
	case strings.Contains(m, "permission denied (publickey)"):
		return "SSH key rejected: check ssh-agent and the key registered on the server"
	case strings.Contains(m, "could not resolve host"):
		return "host not resolved: check the repository URL and the network"
	case strings.Contains(m, "repository not found") || strings.Contains(m, "does not appear to be a git repository"):
		return "repository not found: check the URL and your access rights"
	case strings.Contains(m, "not possible to fast-forward") || strings.Contains(m, "divergent branches"):
		return "branches diverged: merge or rebase in the project directory"
	}
	return ""
}
