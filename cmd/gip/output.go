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
	cli.HandleExitCoder(cli.Exit(msg, exitUsage))
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

// errorHints maps well-known failure messages (lower case substrings of
// git's output) to the next step. The first matching entry wins, so the more
// specific patterns come first.
var errorHints = []struct {
	patterns []string
	hint     string
}{
	{[]string{"could not read username", "terminal prompts disabled", "authentication failed", "could not read password"},
		"authentication required: configure a git credential helper or use an SSH URL"},
	{[]string{"permission denied (publickey)"},
		"SSH key rejected: check ssh-agent and the key registered on the server"},
	{[]string{"host key verification failed"},
		"unknown SSH host key: connect once with ssh to accept it"},
	{[]string{"could not resolve host", "could not resolve hostname"},
		"host not resolved: check the repository URL and the network"},
	{[]string{"connection timed out", "connection refused", "network is unreachable"},
		"server not reachable: check the network, VPN or proxy"},
	{[]string{"ssl certificate problem", "server certificate verification failed"},
		"TLS certificate not trusted: configure http.sslCAInfo for this server"},
	{[]string{"repository not found", "does not appear to be a git repository", "fatal: repository '"},
		"repository not found: check the URL and your access rights"},
	{[]string{"already exists and is not an empty directory"},
		"local_path is not empty: move its content away or point local_path elsewhere"},
	{[]string{"not a git repository"},
		"local_path is not a git repository: fix local_path or remove the directory and run 'gip pull --all'"},
	{[]string{"index.lock", "another git process seems to be running"},
		"another git process is running in this repository (or crashed): wait, or remove .git/index.lock"},
	{[]string{"there is no tracking information", "no tracking information for the current branch"},
		"the current branch has no upstream: git branch --set-upstream-to=origin/<branch>"},
	{[]string{"you are not currently on a branch"},
		"detached HEAD: check out a branch in the project directory"},
	{[]string{"would be overwritten by merge", "would be overwritten by checkout", "commit your changes or stash them"},
		"local changes conflict with the incoming ones: commit or stash them first"},
	{[]string{"not possible to fast-forward", "divergent branches", "refusing to merge unrelated histories"},
		"branches diverged: merge or rebase in the project directory"},
	{[]string{"couldn't find remote ref", "no such ref was fetched"},
		"the upstream branch no longer exists on the remote: switch branch or update its upstream"},
	{[]string{"executable file not found"},
		"command not found: check its name and PATH"},
}

// errorHint suggests the next step for well-known failures.
func errorHint(msg string) string {
	m := strings.ToLower(msg)
	for _, h := range errorHints {
		for _, p := range h.patterns {
			if strings.Contains(m, p) {
				return h.hint
			}
		}
	}
	return ""
}

// commandsWithArgs take positional arguments that users write before the
// flags ("gip init ~/projects -o ~/.gip"): the flag parser stops at the first
// argument, so the flags are moved before it.
var commandsWithArgs = map[string]bool{"init": true}

// moveFlagsBeforeArgs reorders, for the commands in commandsWithArgs, the
// arguments after the command name: flags (with their values) first, then
// the positional arguments. "--" and what follows are left untouched.
func moveFlagsBeforeArgs(args []string, globals []cli.Flag, commands []*cli.Command) []string {
	cmdIdx := commandIndex(args, flagArity(globals))
	if cmdIdx < 0 {
		return args
	}
	cmd := findCommand(commands, args[cmdIdx])
	if cmd == nil || !commandsWithArgs[cmd.Name] {
		return args
	}
	arity := flagArity(cmd.Flags)
	var flags, positional []string
	i := cmdIdx + 1
	for i < len(args) && args[i] != "--" {
		if !isFlagArg(args[i]) {
			positional = append(positional, args[i])
			i++
			continue
		}
		n := flagSpan(args, i, arity[flagName(args[i])])
		flags = append(flags, args[i:i+n]...)
		i += n
	}
	out := append([]string{}, args[:cmdIdx+1]...)
	out = append(out, flags...)
	out = append(out, positional...)
	return append(out, args[i:]...)
}
