package formatter

import (
	"strings"
	"testing"

	"github.com/acesaro/qmd/store"
)

func TestFormatSearchResults(t *testing.T) {
	results := []store.SearchResult{
		{
			Filepath:       "qmd://docs/a.md",
			DisplayPath:    "docs/a.md",
			Title:          "Document A",
			Hash:           "abcdef123456",
			Docid:          "abcdef",
			CollectionName: "docs",
			BodyLength:     100,
			Body:           "This is body text containing quantum physics.",
			Context:        "Root context",
			Score:          0.85,
			Source:         "fts",
		},
	}

	opts := FormatOptions{
		Full:  false,
		Query: "quantum",
	}

	t.Run("JSON format", func(t *testing.T) {
		jsonStr := FormatSearchResults(results, "json", opts)
		if !strings.Contains(jsonStr, `"docid": "#abcdef"`) || !strings.Contains(jsonStr, `"file": "docs/a.md"`) {
			t.Errorf("unexpected JSON output: %s", jsonStr)
		}
	})

	t.Run("CSV format", func(t *testing.T) {
		csvStr := FormatSearchResults(results, "csv", opts)
		if !strings.Contains(csvStr, "docid,score,file,title,context,line,snippet") || !strings.Contains(csvStr, "#abcdef") {
			t.Errorf("unexpected CSV output: %s", csvStr)
		}
	})

	t.Run("Markdown format", func(t *testing.T) {
		mdStr := FormatSearchResults(results, "md", opts)
		if !strings.Contains(mdStr, "# Document A") || !strings.Contains(mdStr, "**file:** `docs/a.md`") {
			t.Errorf("unexpected MD output: %s", mdStr)
		}
	})

	t.Run("XML format", func(t *testing.T) {
		xmlStr := FormatSearchResults(results, "xml", opts)
		if !strings.Contains(xmlStr, `docid="#abcdef"`) || !strings.Contains(xmlStr, `name="docs/a.md"`) {
			t.Errorf("unexpected XML output: %s", xmlStr)
		}
	})
}

func TestFormatDocuments(t *testing.T) {
	files := []MultiGetFile{
		{
			Filepath:    "qmd://docs/a.md",
			DisplayPath: "docs/a.md",
			Title:       "Document A",
			Body:        "This is the body.",
			Context:     "Root context",
			Skipped:     false,
		},
		{
			Filepath:    "qmd://docs/large.md",
			DisplayPath: "docs/large.md",
			Title:       "Large Doc",
			Skipped:     true,
			SkipReason:  "too large",
		},
	}

	t.Run("JSON format", func(t *testing.T) {
		jsonStr := FormatDocuments(files, "json")
		if !strings.Contains(jsonStr, `"file": "docs/a.md"`) || !strings.Contains(jsonStr, `"skipped": true`) {
			t.Errorf("unexpected JSON: %s", jsonStr)
		}
	})

	t.Run("Markdown format", func(t *testing.T) {
		mdStr := FormatDocuments(files, "md")
		if !strings.Contains(mdStr, "## docs/a.md") || !strings.Contains(mdStr, "too large") {
			t.Errorf("unexpected MD: %s", mdStr)
		}
	})
}
