package core

import (
	"context"
	"testing"

	"github.com/enr/clui"
)

func TestCommandRunner_Run_Success(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	err := runner.Run(context.Background(), t.TempDir(), "echo", []string{"hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommandRunner_Run_CommandNotFound(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	err := runner.Run(context.Background(), t.TempDir(), "gip-nonexistent-cmd-xyz", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent command, got nil")
	}
}

func TestCommandRunner_Run_InjectionPrevention(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	err := runner.Run(context.Background(), t.TempDir(), "--malicious", nil)
	if err == nil {
		t.Fatal("expected error for command starting with '-', got nil")
	}
}

func TestCommandRunner_Run_FailingCommand(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	// 'false' is a POSIX standard command that always exits with status 1
	err := runner.Run(context.Background(), t.TempDir(), "false", nil)
	if err == nil {
		t.Fatal("expected error for failing command, got nil")
	}
}

func TestCommandRunner_Run_WorkingDir(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	tmpDir := t.TempDir()
	err := runner.Run(context.Background(), tmpDir, "ls", nil)
	if err != nil {
		t.Fatalf("unexpected error running ls in %s: %v", tmpDir, err)
	}
}

func TestCommandRunner_Run_ContextCancelled(t *testing.T) {
	ui := clui.DefaultClui()
	runner := NewCommandRunner(ui)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runner.Run(ctx, t.TempDir(), "sleep", []string{"10"})
	if err == nil {
		t.Fatal("expected error when context is already cancelled, got nil")
	}
}
