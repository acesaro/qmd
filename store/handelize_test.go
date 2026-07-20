package store

import (
	"testing"
)

func TestHandelize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		err      bool
	}{
		{"README.md", "README.md", false},
		{"MyFile.MD", "MyFile.MD", false},
		{"a/b/c/d.md", "a/b/c/d.md", false},
		{"docs/api/README.md", "docs/api/README.md", false},
		{"hello world.md", "hello-world.md", false},
		{"file (1).md", "file-1.md", false},
		{"foo@bar#baz.md", "foo-bar-baz.md", false},
		{"hello   world.md", "hello-world.md", false},
		{"foo---bar.md", "foo-bar.md", false},
		{"a  -  b.md", "a-b.md", false},
		{"-hello-.md", "hello.md", false},
		{"--test--.md", "test.md", false},
		{"a/-b-/c.md", "a/b/c.md", false},
		{"foo___bar.md", "foo/bar.md", false},
		{"notes___2025___january.md", "notes/2025/january.md", false},
		{"a/b___c/d.md", "a/b/c/d.md", false},
		{"Money Movement Licensing Review - 2025／11／19 10:25 EST - Notes by Gemini.md", "Money-Movement-Licensing-Review-2025-11-19-10-25-EST-Notes-by-Gemini.md", false},
		{"日本語.md", "日本語.md", false},
		{"Зоны и проекты.md", "Зоны-и-проекты.md", false},
		{"café-notes.md", "café-notes.md", false},
		{"naïve.md", "naïve.md", false},
		{"日本語-notes.md", "日本語-notes.md", false},
		{"🐘.md", "1f418.md", false},
		{"🎉.md", "1f389.md", false},
		{"notes 🐘.md", "notes-1f418.md", false},
		{"🐘 elephant.md", "1f418-elephant.md", false},
		{"🐘🎉.md", "1f418-1f389.md", false},
		{"🐘/notes.md", "1f418/notes.md", false},
		{"meeting-2025-01-15.md", "meeting-2025-01-15.md", false},
		{"notes 2025/01/15.md", "notes-2025/01/15.md", false},
		{"call_10:30_AM.md", "call-10-30-AM.md", false},
		{"PROJECT_ABC_v2.0.md", "PROJECT-ABC-v2-0.md", false},
		{"[WIP] Feature Request.md", "WIP-Feature-Request.md", false},
		{"(DRAFT) Proposal v1.md", "DRAFT-Proposal-v1.md", false},
		{"routes/api/auth/$.ts", "routes/api/auth/$.ts", false},
		{"app/routes/$id.tsx", "app/routes/$id.tsx", false},
		{"a//b/c.md", "a/b/c.md", false},
		{"/a/b/", "a/b", false},
		{"///test///", "test", false},
		{"", "", true},
		{"   ", "", true},
		{".md", "", true},
		{"...", "", true},
		{"___", "", true},
		{"a", "a", false},
		{"1", "1", false},
		{"a.md", "a.md", false},
	}

	for _, tc := range tests {
		got, err := Handelize(tc.input)
		if tc.err {
			if err == nil {
				t.Errorf("Handelize(%q) expected error, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("Handelize(%q) unexpected error: %v", tc.input, err)
			} else if got != tc.expected {
				t.Errorf("Handelize(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		}
	}
}
