package store

import "os"

// mkdirAll is a thin os.MkdirAll wrapper to make the helper swappable.
func mkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
