package files

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// Copy source to destination path:
// if destination is a directory, source will be copied in the dir with the file basename.
func Copy(source, destination string) error {
	src := cleanPath(source)
	dst := cleanPath(destination)
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	// no need to check errors on read only file, we already got everything
	// we need from the filesystem, so nothing can go wrong now.
	defer s.Close()

	if IsDir(src) {
		return fmt.Errorf("source path is a directory")
	}
	if IsDir(dst) {
		basename := filepath.Base(src)
		dst = filepath.Join(dst, basename)
	}

	srcInfo, err := s.Stat()
	if err != nil {
		return err
	}
	d, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}

func existsWithError(filepath string) (bool, error) {
	name := cleanPath(filepath)
	_, err := os.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return false, err
		}
		// Windows: error 123 (0x7B) The filename, directory name, or volume label syntax is incorrect.
		if runtime.GOOS == "windows" {
			if e, ok := err.(*os.PathError); ok {
				if en, ok := e.Err.(syscall.Errno); ok {
					if int(en) == 123 {
						return false, err
					}
				}
			}
		}
		return false, err
	}
	return true, nil
}

// Exists reports whether the named file or directory exists and is accessible.
// Returns false if the path does not exist or cannot be accessed (e.g. permission denied).
// Use ExistsWithError to distinguish between these cases.
func Exists(filepath string) bool {
	exist, _ := existsWithError(filepath)
	return exist
}

// ExistsWithError reports whether the named file or directory exists.
// Unlike Exists, it returns the underlying error so callers can distinguish
// between "does not exist" and "exists but inaccessible" (e.g. permission denied).
func ExistsWithError(filepath string) (bool, error) {
	return existsWithError(filepath)
}

// IsDir reports whether d is a directory.
func IsDir(fpath string) bool {
	d := cleanPath(fpath)
	if fi, err := os.Stat(d); err == nil {
		return fi.IsDir()
	}
	return false
}

// IsRegular reports whether filepath is a regular file.
func IsRegular(fpath string) bool {
	f := cleanPath(fpath)
	if fi, err := os.Stat(f); err == nil {
		return fi.Mode().IsRegular()
	}
	return false
}

// Sha1Sum gives the checksum for the given file.
// SHA-1 is not collision-resistant; do not use for security or integrity
// verification where an adversary controls the input — use SHA-256 for that.
func Sha1Sum(fpath string) (string, error) {
	name := cleanPath(fpath)
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sha1 := sha1.New()
	_, err = io.Copy(sha1, f)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha1.Sum(nil)), nil
}

// ReadLines returns a slice containing file lines.
func ReadLines(path string) ([]string, error) {
	lines := []string{}
	file, err := os.Open(cleanPath(path))
	if err != nil {
		return []string{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return []string{}, err
	}
	return lines, err
}

/*
func MD5OfFile(fullpath string) []byte {
	if contents, err := ioutil.ReadFile(fullpath); err == nil {
		var md5sum hash.Hash = md5.New()
		md5sum.Write(contents)
		return md5sum.Sum()
	}
	return nil
}
*/

func cleanPath(filepath string) string {
	return strings.TrimSpace(filepath)
	/*
		withoutSpaces := strings.TrimSpace(filepath)
		if withoutSpaces == "" {
			return ""
		}
		return path.Clean(withoutSpaces)
	*/
}

// EachLineFunc is definition of callback
type EachLineFunc func(line string) error

// EachLine walks lines, calling EachLineFunc for each line of the file.
// All errors that arise visiting lines are filtered by callback function.
func EachLine(path string, walkFn EachLineFunc) error {
	file, err := os.Open(cleanPath(path))
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		err = walkFn(line)
		if err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return err
}

// IsSamePath returns true if two different strings refer to the same file.
// Returns false if either path cannot be resolved (e.g. working directory unavailable).
func IsSamePath(p1 string, p2 string) bool {
	first, err := normalizedPath(p1)
	if err != nil {
		return false
	}
	second, err := normalizedPath(p2)
	if err != nil {
		return false
	}
	return first == second
}

func normalizedPath(p string) (string, error) {
	np := cleanPath(p)
	if np == "" {
		return "", nil
	}
	np = strings.Replace(np, `\`, `/`, -1)
	np, err := filepath.Abs(np)
	if err != nil {
		return "", err
	}
	return np, nil
}

// CopyDir copies a whole directory recursively overwriting contents
func CopyDir(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	fds, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, fd := range fds {
		sourcePath := filepath.Join(src, fd.Name())
		destinationPath := filepath.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = CopyDir(sourcePath, destinationPath); err != nil {
				break
			}
		} else {
			if err = Copy(sourcePath, destinationPath); err != nil {
				break
			}
		}
	}
	return err
}
