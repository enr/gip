package core

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/enr/clui"
)

func TestCommandRunner_Run_Success(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	_, err := runner.Run(context.Background(), t.TempDir(), "echo", []string{"hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommandRunner_Run_CommandNotFound(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	_, err := runner.Run(context.Background(), t.TempDir(), "gip-nonexistent-cmd-xyz", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent command, got nil")
	}
}

func TestCommandRunner_Run_InjectionPrevention(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	_, err := runner.Run(context.Background(), t.TempDir(), "--malicious", nil)
	if err == nil {
		t.Fatal("expected error for command starting with '-', got nil")
	}
}

func TestCommandRunner_Run_FailingCommand(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	// 'false' is a POSIX standard command that always exits with status 1
	_, err := runner.Run(context.Background(), t.TempDir(), "false", nil)
	if err == nil {
		t.Fatal("expected error for failing command, got nil")
	}
}

func TestCommandRunner_Run_WorkingDir(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	tmpDir := t.TempDir()
	_, err := runner.Run(context.Background(), tmpDir, "ls", nil)
	if err != nil {
		t.Fatalf("unexpected error running ls in %s: %v", tmpDir, err)
	}
}

func TestCommandRunner_Run_ContextCancelled(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runner.Run(ctx, t.TempDir(), "sleep", []string{"10"})
	if err == nil {
		t.Fatal("expected error when context is already cancelled, got nil")
	}
}

func TestCommandRunner_Run_Result(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found")
	}
	runner := NewCommandRunner(clui.DefaultClui())
	res, err := runner.Run(context.Background(), t.TempDir(), git, []string{"--version"})
	if err != nil || res.ExitCode != 0 || !strings.HasPrefix(res.Stdout, "git version") {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	res, err = runner.Run(context.Background(), t.TempDir(), git, []string{"rev-parse", "HEAD"})
	if err == nil || res.ExitCode <= 0 || res.Stderr == "" {
		t.Fatalf("failing Run = %+v, %v; want a positive exit code and stderr", res, err)
	}
	res, err = runner.Run(context.Background(), t.TempDir(), "gip-nonexistent-cmd-xyz", nil)
	if err == nil || res.ExitCode != -1 {
		t.Fatalf("not started Run = %+v, %v; want exit code -1", res, err)
	}
}
