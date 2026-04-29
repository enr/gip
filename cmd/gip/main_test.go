package main

import (
	"os/exec"
	"path/filepath"
	"testing"
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
