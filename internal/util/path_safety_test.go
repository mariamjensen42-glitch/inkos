package util

import "testing"

func TestIsSafeBookID(t *testing.T) {
	cases := map[string]bool{
		"a":         true,
		"abc-123":   true,
		"abc_def":   true,
		"":          false,
		"..":        false,
		"foo/bar":   false,
		"foo\\bar":  false,
		"foo bar":   false,
		".hidden":   false,
		"123":       false,
		"foo..bar":  true, // two dots in the middle are fine
	}
	for in, want := range cases {
		got := IsSafeBookID(in)
		if got != want {
			t.Errorf("IsSafeBookID(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSafeRelativePath(t *testing.T) {
	cases := map[string]bool{
		"foo/bar":     true,
		"a":           true,
		"foo/../bar":  false,
		"../escape":   false,
		"/abs":        false,
		"foo/./bar":   true,
	}
	for in, want := range cases {
		got := SafeRelativePath(in)
		if got != want {
			t.Errorf("SafeRelativePath(%q) = %v, want %v", in, got, want)
		}
	}
}
