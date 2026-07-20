package store

import (
	"reflect"
	"testing"
)

func TestVirtualPathHelpers(t *testing.T) {
	t.Run("normalizeVirtualPath", func(t *testing.T) {
		if got := NormalizeVirtualPath("qmd://docs/readme.md"); got != "qmd://docs/readme.md" {
			t.Errorf("expected qmd://docs/readme.md, got %q", got)
		}
		if got := NormalizeVirtualPath("qmd:///docs/readme.md"); got != "qmd://docs/readme.md" {
			t.Errorf("expected qmd://docs/readme.md, got %q", got)
		}
		if got := NormalizeVirtualPath("docs/readme.md"); got != "docs/readme.md" {
			t.Errorf("expected docs/readme.md, got %q", got)
		}
	})

	t.Run("isVirtualPath", func(t *testing.T) {
		if !IsVirtualPath("qmd://docs/readme.md") {
			t.Error("expected true")
		}
		if IsVirtualPath("/tmp/file.md") {
			t.Error("expected false")
		}
	})

	t.Run("parseVirtualPath", func(t *testing.T) {
		got := ParseVirtualPath("qmd://docs/readme.md")
		expected := &VirtualPath{CollectionName: "docs", Path: "readme.md"}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %+v, got %+v", expected, got)
		}

		got = ParseVirtualPath("qmd:///docs/readme.md")
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %+v, got %+v", expected, got)
		}

		if got := ParseVirtualPath("docs/readme.md"); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}
