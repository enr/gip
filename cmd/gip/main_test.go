package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMainExitCode_Failure(t *testing.T) {
	// Build the binary
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "gip")

	buildCmd := exec.Command("go", "build", "-o", binPath)
	buildCmd.Dir = filepath.Dir(binPath) // actually we need to run it in the package dir
	buildCmd.Dir = "."
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build gip: %v", err)
	}

	// Run with a non-existent config file to force an error
	runCmd := exec.Command(binPath, "-f", "non_existent_config.yml", "status")
	err := runCmd.Run()

	if err == nil {
		t.Fatalf("Expected command to fail and exit with non-zero status, but it succeeded (exit code 0)")
	}

	// Check if the error is an ExitError (meaning non-zero exit code)
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 0 {
			t.Fatalf("Expected non-zero exit code, got 0")
		}
	} else {
		t.Fatalf("Expected *exec.ExitError, got %T: %v", err, err)
	}
}

// buildBinary compiles the gip binary into tmpDir and returns its path.
func buildBinary(t *testing.T, tmpDir string) string {
	t.Helper()
	binPath := filepath.Join(tmpDir, "gip")
	out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build gip: %v\n%s", err, out)
	}
	return binPath
}

// makeSlowGit creates a fake git executable in tmpDir that sleeps for sleepSecs seconds.
func makeSlowGit(t *testing.T, tmpDir string, sleepSecs int) {
	t.Helper()
	script := fmt.Sprintf("#!/bin/sh\nexec sleep %d\n", sleepSecs)
	fakeGitPath := filepath.Join(tmpDir, "git")
	if err := os.WriteFile(fakeGitPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create fake git: %v", err)
	}
}

// makeGitRepo creates a directory that looks like a git repo (has a .git subdir).
func makeGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("Failed to create repo dir %s: %v", dir, err)
	}
}

// makeConfig writes a gip YAML config file listing the given repos.
func makeConfig(t *testing.T, path string, repos []string) {
	t.Helper()
	var content string
	for i, repo := range repos {
		content += fmt.Sprintf("- name: repo%d\n  local_path: %s\n  repository: \"https://example.com/repo%d.git\"\n", i+1, repo, i+1)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
}

// TestPullRunsInParallel verifies that pull operates on multiple repos concurrently
// when the default job count is > 1. With two repos each requiring ~2s of git work,
// serial execution takes ~4s while parallel execution should take ~2s.
//
// This test fails before the parallelism fix (serial ≈ 4s > 3.5s threshold).
func TestPullRunsInParallel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test in short mode")
	}

	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)
	makeSlowGit(t, tmpDir, 2)

	repo1 := filepath.Join(tmpDir, "repo1")
	repo2 := filepath.Join(tmpDir, "repo2")
	makeGitRepo(t, repo1)
	makeGitRepo(t, repo2)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1, repo2})

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	start := time.Now()
	// No --jobs flag: relies on the default job count being > 1 after the fix.
	exec.Command(binPath, "-f", configPath, "pull").Run()
	elapsed := time.Since(start)

	// Serial: ~4s. Parallel (≥2 jobs): ~2s. Allow 3.5s slack.
	const threshold = 3500 * time.Millisecond
	if elapsed > threshold {
		t.Fatalf("pull took %v; expected < %v with parallel execution (suggests serial)", elapsed, threshold)
	}
}

// TestPullRespectsTimeout verifies that --timeout=N causes each git operation to be
// killed after N seconds, so a hung remote does not stall the whole command.
//
// This test fails before the timeout fix because urfave/cli rejects --timeout as an
// unknown flag and prints "flag provided but not defined".
func TestPullRespectsTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test in short mode")
	}

	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)
	makeSlowGit(t, tmpDir, 10) // simulates a hung remote

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	start := time.Now()
	cmd := exec.Command(binPath, "-f", configPath, "pull", "--timeout=1")
	out, _ := cmd.CombinedOutput()
	elapsed := time.Since(start)

	// Before the fix --timeout is unrecognized; urfave/cli prints this message.
	if bytes.Contains(out, []byte("flag provided but not defined")) {
		t.Fatal("--timeout flag not recognized; fix not applied")
	}

	// With --timeout=1 the command should return within ~2s (1s timeout + overhead).
	const threshold = 2500 * time.Millisecond
	if elapsed > threshold {
		t.Fatalf("pull --timeout=1 took %v; expected < %v (timeout not respected)", elapsed, threshold)
	}
}

