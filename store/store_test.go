package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/acesaro/qmd/config"
)

func createTestStore(t *testing.T) (*Store, string) {
	tempDir, err := os.MkdirTemp("", "qmd-store-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.sqlite")
	s, err := OpenStore(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open store: %v", err)
	}

	return s, tempDir
}

func TestStoreInitializeSchema(t *testing.T) {
	s, tempDir := createTestStore(t)
	defer s.Close()
	defer os.RemoveAll(tempDir)

	// Check that tables exist
	var name string
	err := s.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='content'").Scan(&name)
	if err != nil || name != "content" {
		t.Errorf("content table does not exist: %v", err)
	}

	err = s.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='documents'").Scan(&name)
	if err != nil || name != "documents" {
		t.Errorf("documents table does not exist: %v", err)
	}
}

func TestStoreSyncConfigToDb(t *testing.T) {
	s, tempDir := createTestStore(t)
	defer s.Close()
	defer os.RemoveAll(tempDir)

	incl := true
	cfg := &config.CollectionConfig{
		Collections: map[string]config.Collection{
			"notes": {
				Path:             "/path/to/notes",
				Pattern:          "**/*.md",
				IncludeByDefault: &incl,
				Context: map[string]string{
					"/": "Global notes context",
				},
			},
		},
	}

	err := s.SyncConfigToDb(cfg)
	if err != nil {
		t.Fatalf("SyncConfigToDb failed: %v", err)
	}

	var path, pattern, context string
	var includeByDefault int
	err = s.DB.QueryRow("SELECT path, pattern, include_by_default, context FROM store_collections WHERE name='notes'").
		Scan(&path, &pattern, &includeByDefault, &context)

	if err != nil {
		t.Fatalf("failed to query store_collections: %v", err)
	}

	if path != "/path/to/notes" || pattern != "**/*.md" || includeByDefault != 1 {
		t.Errorf("unexpected collection sync: path=%q, pattern=%q, include_by_default=%d", path, pattern, includeByDefault)
	}

	if !strings.Contains(context, `"Global notes context"`) {
		t.Errorf("expected context to contain Global notes context, got %q", context)
	}
}

func TestStoreReindexCollection(t *testing.T) {
	s, tempDir := createTestStore(t)
	defer s.Close()
	defer os.RemoveAll(tempDir)

	// Create a mock collection directory
	collDir := filepath.Join(tempDir, "my-notes")
	if err := os.MkdirAll(collDir, 0755); err != nil {
		t.Fatalf("failed to create collection dir: %v", err)
	}

	// Write mock markdown files
	fileA := filepath.Join(collDir, "a.md")
	if err := os.WriteFile(fileA, []byte("# Title A\n\nContent for document A.\n"), 0644); err != nil {
		t.Fatalf("failed to write a.md: %v", err)
	}

	fileB := filepath.Join(collDir, "sub/b.md")
	if err := os.MkdirAll(filepath.Dir(fileB), 0755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("# Title B\n\nContent for document B.\n"), 0644); err != nil {
		t.Fatalf("failed to write b.md: %v", err)
	}

	// Reindex collection
	res, err := s.ReindexCollection(collDir, "**/*.md", "notes", nil, nil)
	if err != nil {
		t.Fatalf("ReindexCollection failed: %v", err)
	}

	if res.Indexed != 2 || res.Updated != 0 {
		t.Errorf("expected indexed=2, updated=0; got indexed=%d, updated=%d", res.Indexed, res.Updated)
	}

	// Query documents
	var count int
	err = s.DB.QueryRow("SELECT COUNT(*) FROM documents WHERE collection='notes' AND active=1").Scan(&count)
	if err != nil || count != 2 {
		t.Errorf("expected 2 active documents, got %d (err: %v)", count, err)
	}

	// Reindex again, should be unchanged
	res2, err := s.ReindexCollection(collDir, "**/*.md", "notes", nil, nil)
	if err != nil {
		t.Fatalf("ReindexCollection failed: %v", err)
	}
	if res2.Unchanged != 2 || res2.Indexed != 0 {
		t.Errorf("expected unchanged=2; got indexed=%d, unchanged=%d", res2.Indexed, res2.Unchanged)
	}

	// Modify a file
	if err := os.WriteFile(fileA, []byte("# Title A (Updated)\n\nContent for document A.\n"), 0644); err != nil {
		t.Fatalf("failed to modify a.md: %v", err)
	}

	res3, err := s.ReindexCollection(collDir, "**/*.md", "notes", nil, nil)
	if err != nil {
		t.Fatalf("ReindexCollection failed: %v", err)
	}
	if res3.Updated != 1 || res3.Unchanged != 1 {
		t.Errorf("expected updated=1, unchanged=1; got updated=%d, unchanged=%d", res3.Updated, res3.Unchanged)
	}

	// Remove a file
	if err := os.Remove(fileB); err != nil {
		t.Fatalf("failed to remove b.md: %v", err)
	}

	res4, err := s.ReindexCollection(collDir, "**/*.md", "notes", nil, nil)
	if err != nil {
		t.Fatalf("ReindexCollection failed: %v", err)
	}
	if res4.Removed != 1 {
		t.Errorf("expected removed=1, got %d", res4.Removed)
	}
}

func TestStoreSearchFTS(t *testing.T) {
	s, tempDir := createTestStore(t)
	defer s.Close()
	defer os.RemoveAll(tempDir)

	collDir := filepath.Join(tempDir, "my-docs")
	if err := os.MkdirAll(collDir, 0755); err != nil {
		t.Fatalf("failed to create collection dir: %v", err)
	}

	fileA := filepath.Join(collDir, "a.md")
	if err := os.WriteFile(fileA, []byte("# Hello World\n\nThis is a sample document containing quantum mechanics.\n"), 0644); err != nil {
		t.Fatalf("failed to write a.md: %v", err)
	}

	_, err := s.ReindexCollection(collDir, "**/*.md", "docs", nil, nil)
	if err != nil {
		t.Fatalf("ReindexCollection failed: %v", err)
	}

	// FTS Search
	results, err := s.SearchFTS("quantum", 10, "docs")
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Title != "Hello World" {
		t.Errorf("expected title 'Hello World', got %q", results[0].Title)
	}

	if results[0].Source != "fts" {
		t.Errorf("expected source 'fts', got %q", results[0].Source)
	}
}

func TestStoreGetContextForFile(t *testing.T) {
	s, tempDir := createTestStore(t)
	defer s.Close()
	defer os.RemoveAll(tempDir)

	collDir := filepath.Join(tempDir, "my-notes")
	if err := os.MkdirAll(collDir, 0755); err != nil {
		t.Fatalf("failed to create collection dir: %v", err)
	}

	// Set configuration sync
	incl := true
	cfg := &config.CollectionConfig{
		Collections: map[string]config.Collection{
			"notes": {
				Path:             collDir,
				Pattern:          "**/*.md",
				IncludeByDefault: &incl,
				Context: map[string]string{
					"/":    "Notes root context",
					"/sub": "Notes subfolder context",
				},
			},
		},
		GlobalContext: "App context",
	}
	s.SyncConfigToDb(cfg)

	// Add global context to store_config for verification
	s.DB.Exec("INSERT OR REPLACE INTO store_config (key, value) VALUES ('global_context', 'App context')")

	fileA := filepath.Join(collDir, "sub/a.md")
	if err := os.MkdirAll(filepath.Dir(fileA), 0755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}
	if err := os.WriteFile(fileA, []byte("# Doc A\n\nContent.\n"), 0644); err != nil {
		t.Fatalf("failed to write a.md: %v", err)
	}

	_, err := s.ReindexCollection(collDir, "**/*.md", "notes", nil, nil)
	if err != nil {
		t.Fatalf("ReindexCollection failed: %v", err)
	}

	ctx := s.GetContextForFile("qmd://notes/sub/a.md")
	expected := "App context\n\nNotes root context\n\nNotes subfolder context"
	if ctx != expected {
		t.Errorf("expected context %q, got %q", expected, ctx)
	}
}
