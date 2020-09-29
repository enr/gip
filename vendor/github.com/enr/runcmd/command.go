package runcmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/extemporalgenome/slug"
)

// Command is the launched command, it wraps the process.
type Command struct {
	Name string
	// the executable
	Exe string
	// args passed to executable
	Args []string
	// the command is run in a shell, that is prepend `/bin/sh -c` or `cmd /C` to the command line
	CommandLine string
	WorkingDir  string
	// only for *nix: if not set, runcmd uses env[$SHELL] or defaults to /bin/sh
	ForceShell string
	// custom environment variables. these are overwritten from .env file if UseEnv is true
	Env Env
	// only for *nix: if true .profile file in the working dir is sourced
	UseProfile bool // dovrebbe essere Profile string : path
	// only for *nix: if true .env file in the working dir is used to initialize env vars
	UseEnv bool // dovrebbe essere EnvFile string: path
	// used only if command is started as process
	Logfile string
	// the underlying process
	Process *os.Process
}

func (c *Command) String() string {
	return fmt.Sprintf("%s# %s", c.WorkingDir, c.FullCommand())
}

// FullCommand returns the full command line string.
func (c *Command) FullCommand() string {
	if c.Exe == "" && c.CommandLine == "" {
		return ""
	}
	if c.Exe == "" {
		c.useShell()
	}
	return strings.TrimSpace(c.Exe + " " + strings.Join(c.Args, " "))
}

// GetName get the name of the command.
func (c *Command) GetName() string {
	if c.Name != "" {
		return c.Name
	}
	var dirtyCommandName string
	if c.CommandLine != "" {
		dirtyCommandName = c.CommandLine
	} else {
		dirtyCommandName = fmt.Sprintf("%s%v", c.Exe, c.Args)
	}
	return slug.Slug(dirtyCommandName)
}

// GetLogfile get the full path to the file containing the process output.
func (c *Command) GetLogfile() string {
	if c.Logfile != "" {
		return c.Logfile
	}
	ln := c.GetName()
	if len(ln) > 80 {
		ln = fmt.Sprintf(`%s-%d`, ln[0:60], time.Now().UnixNano())
	}
	logname := fmt.Sprintf("runcmd-%s.log", ln)
	fullpath := path.Join(os.TempDir(), logname)
	return fullpath
}

// Run starts the specified command and waits for it to complete.
func (c *Command) Run() *ExecResult {
	var bout, berr bytes.Buffer
	outputs := &outputs{
		out: &bout,
		err: &berr,
	}
	result := &ExecResult{
		fullCommand: c.FullCommand(),
		outputs:     outputs,
	}
	cmd, err := c.buildCmd()
	if err != nil {
		result.err = err
		return result
	}
	cmd.Stdout = &bout
	cmd.Stderr = &berr
	if c.WorkingDir != "" {
		cmd.Dir = c.WorkingDir
	}

	if c.UseEnv {
		flagEnv := filepath.Join(cmd.Dir, ".env")
		env, _ := readEnv(flagEnv)
		cmd.Env = env.asArray()
	} else if len(c.Env) > 0 {
		cmd.Env = c.Env.asArray()
	}
	// On Windows, clearing the environment,
	// or having missing environment variables, may lead to powershell errors.
	if runtime.GOOS == "windows" {
		cmd.Env = mergeEnvironment(cmd.Env)
	}

	if err := cmd.Run(); err != nil {
		result.err = err
		return result
	}
	return result
}

func (c *Command) buildCmd() (*exec.Cmd, error) {
	if c.Exe == "" && c.CommandLine == "" {
		return nil, fmt.Errorf("error creating command: no Exe nor CommandLine")
	}
	if c.Exe != "" {
		return exec.Command(c.Exe, c.Args...), nil
	}
	// if commandline, use shell
	c.useShell()
	cmd := exec.Command(c.Exe, c.Args...)
	return cmd, nil
}
