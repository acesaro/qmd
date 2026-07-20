package store

import (
	"testing"
)

func TestGlobMatching(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		{"**/*.md", "a.md", true},
		{"**/*.md", "docs/a.md", true},
		{"**/*.md", "docs/sub/a.md", true},
		{"**/*.md", "a.txt", false},
		{"docs/*.md", "docs/a.md", true},
		{"docs/*.md", "docs/sub/a.md", false},
		{"docs/**/*.md", "docs/a.md", true},
		{"docs/**/*.md", "docs/sub/a.md", true},
		{"docs/**/*.md", "other/a.md", false},
		{"*.*", "a.md", true},
		{"*.*", "a/b.md", false},
	}

	for _, tc := range tests {
		got := MatchPath(tc.pattern, tc.path)
		if got != tc.match {
			t.Errorf("MatchPath(%q, %q) = %t; want %t", tc.pattern, tc.path, got, tc.match)
		}
	}
}
