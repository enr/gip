//go:build windows
// +build windows

package files

import "os"

func isSymlink(p string) bool {
	candidate := cleanPath(p)
	if candidate == "" {
		return false
	}
	fi, err := os.Lstat(candidate)
	if err != nil {
		return false
	}
	return (fi.Mode()&os.ModeSymlink == os.ModeSymlink)
}
