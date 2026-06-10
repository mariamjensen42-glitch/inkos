package api

import (
	"crypto/sha1"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/narcooo/inkos/internal/model"
)

// nowISO returns the current time in ISO 8601 (RFC3339) format.
func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }

// bookIDFromTitle derives a safe filesystem id from a free-form title.
// Pure-ASCII titles are slugified. Non-ASCII titles fall back to a
// short SHA-1 hex hash so the function works for any language.
func bookIDFromTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return ""
	}
	// Slugify ASCII letters/digits.
	var sb strings.Builder
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z':
			sb.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			sb.WriteRune(r + 32)
		case r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r == '-' || r == '_':
			sb.WriteRune(r)
		case unicode.IsSpace(r) || r == '.' || r == ',' || r == '!' || r == '?' || r == '|' || r == '/' || r == '\\' || r == ':' || r == '；' || r == '，' || r == '。':
			sb.WriteRune('-')
		}
	}
	slug := strings.Trim(sb.String(), "-_")
	if slug != "" {
		// Ensure it starts with [a-z]; prefix "b-" if needed.
		if !regexp.MustCompile(`^[a-z]`).MatchString(slug) {
			slug = "b-" + slug
		}
		if len(slug) > 64 {
			slug = slug[:64]
		}
		return slug
	}
	// Fallback: SHA-1 hex of original title (8 chars).
	h := sha1.Sum([]byte(t))
	return "t-" + hex.EncodeToString(h[:])[:8]
}

// randomHex returns n random bytes as hex.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// writeFileAtomic writes data to a file via temp + rename.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// createBookOnDisk creates the books/<id>/ skeleton and writes book.json.
func createBookOnDisk(root string, cfg *model.BookConfig) error {
	dirs := []string{
		filepath.Join(root, "books", cfg.ID),
		filepath.Join(root, "books", cfg.ID, "chapters"),
		filepath.Join(root, "books", cfg.ID, "story"),
		filepath.Join(root, "books", cfg.ID, "story", "state"),
		filepath.Join(root, "books", cfg.ID, "story", "runtime"),
		filepath.Join(root, "books", cfg.ID, "story", "snapshots"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}
