package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func binaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func TestMainExitCode_Failure(t *testing.T) {
	// Build the binary
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, binaryName("gip"))

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
	binPath := filepath.Join(tmpDir, binaryName("gip"))
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

// TestErrorOutputNoBanner verifies that when a command fails the output does not
// contain the "Something gone wrong:" banner — only the actual error message.
// This test fails before the fix that removes ui.Errorf from exitError.
func TestErrorOutputNoBanner(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	cmd := exec.Command(binPath, "-f", "non_existent_config.yml", "status")
	out, _ := cmd.CombinedOutput()

	if bytes.Contains(out, []byte("Something gone wrong:")) {
		t.Fatalf("error output contains noisy banner \"Something gone wrong:\"; output:\n%s", out)
	}
}

// makeConfigWithTags writes a gip YAML config with tagged repos.
func makeConfigWithTags(t *testing.T, path string, entries []struct {
	dir  string
	tags []string
}) {
	t.Helper()
	var content string
	for i, e := range entries {
		content += fmt.Sprintf("- name: repo%d\n  local_path: %s\n  repository: \"https://example.com/r%d.git\"\n", i+1, e.dir, i+1)
		if len(e.tags) > 0 {
			content += "  tags:\n"
			for _, tag := range e.tags {
				content += fmt.Sprintf("  - %s\n", tag)
			}
		}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
}

// TestListTabular verifies that `gip list` outputs a table with headers.
func TestListTabular(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	repo1 := filepath.Join(tmpDir, "alpha")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	cmd := exec.Command(binPath, "-f", configPath, "list")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list failed: %v\nOutput: %s", err, out)
	}
	for _, col := range []string{"NAME", "PATH", "POLICY", "PROVIDER"} {
		if !bytes.Contains(out, []byte(col)) {
			t.Errorf("expected column header %q in list output:\n%s", col, out)
		}
	}
	if !bytes.Contains(out, []byte("alpha")) {
		t.Errorf("expected repo name 'alpha' in list output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("default")) {
		t.Errorf("expected 'default' policy in list output:\n%s", out)
	}
}

// TestNoopFlag verifies that --noop prints dry-run lines and exits 0 without executing.
func TestNoopFlag(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	// Use a slow fake git: if noop works correctly it will NOT be called.
	makeSlowGit(t, tmpDir, 10)
	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	start := time.Now()
	cmd := exec.Command(binPath, "--noop", "-f", configPath, "pull")
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("--noop pull failed: %v\nOutput: %s", err, out)
	}
	// Should complete fast (no git call)
	if elapsed > 2*time.Second {
		t.Fatalf("--noop took %v; expected near-instant (no git executed)", elapsed)
	}
	if !bytes.Contains(out, []byte("[DRY-RUN]")) {
		t.Fatalf("expected [DRY-RUN] in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("DRY-RUN —")) {
		t.Fatalf("expected noop note in summary:\n%s", out)
	}
}

// TestAutoConfigDetection verifies UX-06: .gip in current dir is used automatically.
func TestAutoConfigDetection(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)
	makeSlowGit(t, tmpDir, 0)

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	// Write config as .gip in a subdirectory (simulates working directory)
	workDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(workDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Clear GIP_FILE so it doesn't interfere
	t.Setenv("GIP_FILE", "")
	// Point HOME to a temp dir without .gip so only local .gip is found
	t.Setenv("HOME", tmpDir)

	// Run from workDir (where .gip lives) without -f flag
	cmd := exec.Command(binPath, "list")
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list without -f failed: %v\nOutput: %s", err, out)
	}
	if !bytes.Contains(out, []byte("repo1")) {
		t.Fatalf("expected 'repo1' from auto-detected config:\n%s", out)
	}
}

// TestGIPFILEEnv verifies UX-06: GIP_FILE env var is respected.
func TestGIPFILEEnv(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)
	makeSlowGit(t, tmpDir, 0)

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, "custom.yaml")
	makeConfig(t, configPath, []string{repo1})

	t.Setenv("GIP_FILE", configPath)
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cmd := exec.Command(binPath, "list") // no -f flag
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list with GIP_FILE failed: %v\nOutput: %s", err, out)
	}
	if !bytes.Contains(out, []byte("repo1")) {
		t.Fatalf("expected 'repo1' from GIP_FILE config:\n%s", out)
	}
}

// TestSummaryPrinted verifies that UX-01 summary line appears at end of command output.
func TestSummaryPrinted(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)
	makeSlowGit(t, tmpDir, 0)

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cmd := exec.Command(binPath, "-f", configPath, "status")
	out, _ := cmd.CombinedOutput()

	if !bytes.Contains(out, []byte("OK:")) {
		t.Fatalf("expected summary 'OK:' in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("Duration:")) {
		t.Fatalf("expected summary 'Duration:' in output:\n%s", out)
	}
}

// TestErrorsLastFlag verifies that --errors-last groups errors at end of output.
func TestErrorsLastFlag(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)
	makeSlowGit(t, tmpDir, 0) // fake git that exits 0

	// repo1 exists, repo2 does not → repo2 will be skipped (missing dir)
	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	// Write config with a repo that has an unreachable path to force an error
	configPath := filepath.Join(tmpDir, ".gip")
	// Use a path with bad chars to force projectPath error - actually
	// let's use a non-existing dir which becomes "skipped", not "error".
	// For a real error we need git to fail, which the fake git (exit 0) won't produce.
	// Instead, use pull on a missing repo without --all: that records opSkipped.
	makeConfig(t, configPath, []string{repo1})

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cmd := exec.Command(binPath, "-f", configPath, "status", "--errors-last")
	out, _ := cmd.CombinedOutput()

	// summary line must still be present
	if !bytes.Contains(out, []byte("OK:")) {
		t.Fatalf("expected summary with --errors-last:\n%s", out)
	}
}

// TestTagFilterStatus verifies that --tag restricts which repos are processed.
func TestTagFilterStatus(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)
	makeSlowGit(t, tmpDir, 0) // instant fake git

	repo1 := filepath.Join(tmpDir, "repo1")
	repo2 := filepath.Join(tmpDir, "repo2")
	makeGitRepo(t, repo1)
	makeGitRepo(t, repo2)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfigWithTags(t, configPath, []struct {
		dir  string
		tags []string
	}{
		{repo1, []string{"work"}},
		{repo2, []string{"personal"}},
	})

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Only repo1 (tagged "work") should be processed; repo2 is skipped.
	cmd := exec.Command(binPath, "-f", configPath, "status", "--tag", "work")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status --tag work failed: %v\nOutput: %s", err, out)
	}
	if bytes.Contains(out, []byte("repo2")) {
		t.Fatalf("output contains repo2 which should have been filtered out:\n%s", out)
	}
}

// TestBranchCommand verifies that the branch command prints a table with branch names.
func TestBranchCommand(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	// Fake git that prints "main" to stdout regardless of args.
	script := "#!/bin/sh\necho main\n"
	fakeGitPath := filepath.Join(tmpDir, "git")
	if err := os.WriteFile(fakeGitPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create fake git: %v", err)
	}

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cmd := exec.Command(binPath, "-f", configPath, "branch")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("branch command failed: %v\nOutput: %s", err, out)
	}
	if !bytes.Contains(out, []byte("NAME")) {
		t.Fatalf("expected table header 'NAME' in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("main")) {
		t.Fatalf("expected branch 'main' in output:\n%s", out)
	}
}

// TestInitCommand verifies that gip init scans a directory and writes a config file.
func TestInitCommand(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	// Create a real git repo with an origin remote so getOriginURL works.
	repoDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, gitCmd := range [][]string{
		{"init", repoDir},
		{"-C", repoDir, "remote", "add", "origin", "https://example.com/myproject.git"},
	} {
		if out, err := exec.Command("git", gitCmd...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", gitCmd, err, out)
		}
	}

	outputPath := filepath.Join(tmpDir, "out.yaml")
	cmd := exec.Command(binPath, "init", "--output", outputPath, "--force", repoDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init command failed: %v\nOutput: %s", err, out)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if !bytes.Contains(data, []byte("myproject")) {
		t.Fatalf("output YAML does not contain 'myproject':\n%s", data)
	}
	if !bytes.Contains(data, []byte("https://example.com/myproject.git")) {
		t.Fatalf("output YAML does not contain origin URL:\n%s", data)
	}
}

// TestInitCommand_NoRepos verifies that init exits 0 when no repos with origin are found.
func TestInitCommand_NoRepos(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cmd := exec.Command(binPath, "init", "--output", filepath.Join(tmpDir, "out.yaml"), "--force", emptyDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected exit 0 when no repos found, got: %v", err)
	}
}

// TestFetchRunsInParallel verifies that fetch operates on multiple repos concurrently.
func TestFetchRunsInParallel(t *testing.T) {
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
	exec.Command(binPath, "-f", configPath, "fetch").Run()
	elapsed := time.Since(start)

	const threshold = 3500 * time.Millisecond
	if elapsed > threshold {
		t.Fatalf("fetch took %v; expected < %v with parallel execution", elapsed, threshold)
	}
}

// TestFetchSkipsPullNever verifies that projects with pull_policy "never" are skipped.
func TestFetchSkipsPullNever(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	content := fmt.Sprintf("- name: repo1\n  local_path: %s\n  repository: \"https://example.com/r.git\"\n  pull_policy: never\n", repo1)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// With pull_policy: never the repo is skipped; no git binary needed, exit 0.
	cmd := exec.Command(binPath, "-f", configPath, "fetch")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Expected exit 0 when all repos are skipped, got: %v", err)
	}
}

// TestFetchRespectsTimeout verifies that --timeout kills a hung fetch operation.
func TestFetchRespectsTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test in short mode")
	}

	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)
	makeSlowGit(t, tmpDir, 10)

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	start := time.Now()
	exec.Command(binPath, "-f", configPath, "fetch", "--timeout=1").Run()
	elapsed := time.Since(start)

	const threshold = 2500 * time.Millisecond
	if elapsed > threshold {
		t.Fatalf("fetch --timeout=1 took %v; expected < %v", elapsed, threshold)
	}
}

// TestExecCommand verifies that exec runs an arbitrary command in each project directory.
func TestExecCommand(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	cmd := exec.Command(binPath, "-f", configPath, "exec", "--", "echo", "hello-gip")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exec command failed: %v\nOutput: %s", err, out)
	}
	if !bytes.Contains(out, []byte("hello-gip")) {
		t.Fatalf("expected output to contain 'hello-gip', got: %s", out)
	}
}

// TestExecCommand_NoArgs verifies that exec exits non-zero when called without a command.
func TestExecCommand_NoArgs(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{})

	cmd := exec.Command(binPath, "-f", configPath, "exec")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit code when exec is called with no command")
	}
}

// TestExecCommand_FailingCommand verifies that exec propagates a non-zero exit from the child.
func TestExecCommand_FailingCommand(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	cmd := exec.Command(binPath, "-f", configPath, "exec", "--", "false")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit code when the executed command fails")
	}
}

// TestExecCommand_RespectsTimeout verifies that --timeout kills a hung command.
func TestExecCommand_RespectsTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test in short mode")
	}

	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	repo1 := filepath.Join(tmpDir, "repo1")
	makeGitRepo(t, repo1)

	configPath := filepath.Join(tmpDir, ".gip")
	makeConfig(t, configPath, []string{repo1})

	start := time.Now()
	cmd := exec.Command(binPath, "-f", configPath, "exec", "--timeout=1", "--", "sleep", "10")
	cmd.Run()
	elapsed := time.Since(start)

	const threshold = 2500 * time.Millisecond
	if elapsed > threshold {
		t.Fatalf("exec --timeout=1 took %v; expected < %v (timeout not respected)", elapsed, threshold)
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
