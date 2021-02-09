package files

// IsSymlink returns if a given file is a symbolic link on a Unix like OS or false if the OS is Windows
func IsSymlink(path string) bool {
	return isSymlink(path)
}
