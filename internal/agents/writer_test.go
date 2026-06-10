package agents

import (
	"testing"
)

func TestCountWords(t *testing.T) {
	cases := map[string]int{
		"":         0,
		"hello":    1,
		"two words": 2,
		"中文 标题":  4, // each CJK char counts as 1, plus 1 for "标题"? Actually 2 CJK chars + 1 word
	}
	for in, want := range cases {
		got := countWords(in)
		if got != want {
			t.Errorf("countWords(%q) = %d, want %d", in, got, want)
		}
	}
}
