// Package util provides path-safety and book-id validation helpers.
package util

import (
	"path/filepath"
	"regexp"
	"strings"
)

// safeBookIDPattern matches a book id allowed on disk. Lowercase ASCII,
// digits, underscore, dash, dot; must start with a letter. Dots in the
// middle are allowed (so existing book ids like "foo..bar" still work).
var safeBookIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

// IsSafeBookID reports whether the given book id is safe to use in a
// filesystem path. This guards against path traversal, control chars,
// and other hazards used to build books/<id>/.
func IsSafeBookID(id string) bool {
	if id == "" {
		return false
	}
	if strings.ContainsRune(id, 0) {
		return false
	}
	if strings.ContainsAny(id, "/\\") {
		return false
	}
	if strings.HasPrefix(id, ".") {
		return false
	}
	if filepath.Separator != '/' && strings.ContainsRune(id, filepath.Separator) {
		return false
	}
	return safeBookIDPattern.MatchString(id)
}

// SafeRelativePath verifies that a relative path under project root does
// not escape (no .. segments, no absolute prefix, no NUL).
func SafeRelativePath(rel string) bool {
	if rel == "" {
		return false
	}
	if strings.ContainsRune(rel, 0) {
		return false
	}
	if filepath.IsAbs(rel) {
		return false
	}
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		if seg == ".." {
			return false
		}
	}
	// Also check on the slash separator for cross-platform safety.
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}
