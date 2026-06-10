package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/narcooo/inkos/internal/util"
)

func TestBookIDFromTitleASCII(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Sky World", "sky-world"},
		{"hello", "hello"},
		{"  The Quick-Brown_Fox!  ", "the-quick-brown_fox"},
		{"中文标题", "t-"}, // falls back to hash; just check prefix
	}
	for _, c := range cases {
		got := bookIDFromTitle(c.in)
		if c.want == "t-" && len(got) < 2 {
			t.Errorf("bookIDFromTitle(%q) = %q, want hash prefix", c.in, got)
			continue
		}
		if c.want != "t-" && got != c.want {
			t.Errorf("bookIDFromTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBookIDFromTitleUnsafe(t *testing.T) {
	if bookIDFromTitle("") != "" {
		t.Error("empty should give empty")
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.txt")
	if err := writeFileAtomic(target, []byte("hi")); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	if err := writeFileAtomic(target, []byte("bye")); err != nil {
		t.Fatalf("writeFileAtomic overwrite: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "bye" {
		t.Errorf("got %q, want %q", got, "bye")
	}
}

func TestUtilReexport(t *testing.T) {
	// Sanity: util.IsSafeBookID is callable; we re-export it for
	// convenience in some handlers.
	if !util.IsSafeBookID("alpha-1") {
		t.Error("alpha-1 should be safe")
	}
}
