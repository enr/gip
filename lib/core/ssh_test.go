package core

import (
	"context"
	"os/exec"
	"testing"
)

func newTestGitWrapper(t *testing.T) defaultGitWrapper {
	t.Helper()
	git, err := gitExecutablePath()
	if err != nil {
		t.Skip("git not available")
	}
	return defaultGitWrapper{git: git}
}

func TestSSHCommand_Default(t *testing.T) {
	g := newTestGitWrapper(t)
	t.Setenv("GIT_SSH_COMMAND", "")
	t.Setenv("GIT_SSH", "")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if got := g.sshCommand(context.Background(), t.TempDir()); got != "ssh -o BatchMode=yes" {
		t.Errorf("sshCommand = %q, want default", got)
	}
}

func TestSSHCommand_PreservesEnv(t *testing.T) {
	g := newTestGitWrapper(t)
	t.Setenv("GIT_SSH_COMMAND", "ssh -i key")
	if got := g.sshCommand(context.Background(), t.TempDir()); got != "ssh -i key -o BatchMode=yes" {
		t.Errorf("sshCommand = %q", got)
	}
}

func TestSSHCommand_LeavesGitSSHAlone(t *testing.T) {
	g := newTestGitWrapper(t)
	t.Setenv("GIT_SSH_COMMAND", "")
	t.Setenv("GIT_SSH", "plink")
	if got := g.sshCommand(context.Background(), t.TempDir()); got != "" {
		t.Errorf("sshCommand = %q, want empty so GIT_SSH is used", got)
	}
}

func TestSSHCommand_PreservesCoreSSHCommand(t *testing.T) {
	g := newTestGitWrapper(t)
	t.Setenv("GIT_SSH_COMMAND", "")
	t.Setenv("GIT_SSH", "")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", dir},
		{"-C", dir, "config", "core.sshCommand", "ssh -i ~/.ssh/work_key"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if got := g.sshCommand(context.Background(), dir); got != "ssh -i ~/.ssh/work_key -o BatchMode=yes" {
		t.Errorf("sshCommand = %q", got)
	}
}
