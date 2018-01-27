package environment

import (
	"os"
	"os/exec"
	"strings"

	"github.com/mitchellh/go-homedir"
)

// GetenvEitherCase returns env var.
// Got from http://golang.org/src/pkg/net/http/transport.go
func GetenvEitherCase(k string) string {
	if k == "" {
		return ""
	}
	if v := os.Getenv(k); v != "" {
		return v
	}
	if v := os.Getenv(strings.ToUpper(k)); v != "" {
		return v
	}
	return os.Getenv(strings.ToLower(k))
}

// UserHome returns the home directory for the executing user.
func UserHome() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return home, nil
}

func whichExecutable(exe string) (string, error) {
	path, err := exec.LookPath(exe)
	if err != nil {
		if _, ok := err.(*exec.Error); ok {
			return "", nil
		}
		return "", err
	}
	return path, nil
}

// Which returns the full path to executable or an empty string if executable is not found.
func Which(exe string) string {
	path, _ := whichExecutable(exe)
	return path
}
