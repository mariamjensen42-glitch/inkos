package store

import (
	"os"
	"path/filepath"
)

// mkdirAll is a thin os.MkdirAll wrapper to make the helper swappable.
func mkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// WriteFileAtomic writes data to path via a temp file and rename. The
// parent directory is created with 0o755 if missing.
func WriteFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
