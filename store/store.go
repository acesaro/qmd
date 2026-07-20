package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/acesaro/qmd/config"
	"github.com/acesaro/qmd/db"
)

type Store struct {
	DB *sql.DB
}

type SearchResult struct {
	Filepath       string  `json:"filepath"`
	DisplayPath    string  `json:"displayPath"`
	Title          string  `json:"title"`
	Hash           string  `json:"hash"`
	Docid          string  `json:"docid"`
	CollectionName string  `json:"collectionName"`
	ModifiedAt     string  `json:"modifiedAt"`
	BodyLength     int     `json:"bodyLength"`
	Body           string  `json:"body"`
	Context        string  `json:"context"`
	Score          float64 `json:"score"`
	Source         string  `json:"source"`
}

type ReindexProgress struct {
	File    string
	Current int
	Total   int
}

type ReindexResult struct {
	Indexed         int
	Updated         int
	Unchanged       int
	Removed         int
	OrphanedCleaned int
}

func OpenStore(dbPath string) (*Store, error) {
	conn, err := db.OpenDatabase(dbPath)
	if err != nil {
		return nil, err
	}

	store := &Store{DB: conn}
	if err := store.InitializeSchema(); err != nil {
		conn.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func (s *Store) InitializeSchema() error {
	// Foreign keys ON
	_, err := s.DB.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return err
	}

	// Schema definitions
	queries := []string{
		`CREATE TABLE IF NOT EXISTS content (
			hash TEXT PRIMARY KEY,
			doc TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collection TEXT NOT NULL,
			path TEXT NOT NULL,
			title TEXT NOT NULL,
			hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			modified_at TEXT NOT NULL,
			active INTEGER NOT NULL DEFAULT 1,
			FOREIGN KEY (hash) REFERENCES content(hash) ON DELETE CASCADE,
			UNIQUE(collection, path)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_collection ON documents(collection, active)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(hash)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_path ON documents(path, active)`,
		`CREATE TABLE IF NOT EXISTS store_collections (
			name TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			pattern TEXT NOT NULL DEFAULT '**/*.md',
			ignore_patterns TEXT,
			include_by_default INTEGER DEFAULT 1,
			update_command TEXT,
			context TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS store_config (
			key TEXT PRIMARY KEY,
			value TEXT
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
			filepath, title, body,
			tokenize='porter unicode61'
		)`,
	}

	for _, q := range queries {
		if _, err := s.DB.Exec(q); err != nil {
			return fmt.Errorf("failed schema execution: %w", err)
		}
	}

	// Triggers
	triggers := []struct {
		name string
		body string
	}{
		{
			"documents_ai",
			`CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents
			WHEN new.active = 1
			BEGIN
				INSERT INTO documents_fts(rowid, filepath, title, body)
				SELECT
					new.id,
					new.collection || '/' || new.path,
					new.title,
					(SELECT doc FROM content WHERE hash = new.hash)
				WHERE new.active = 1;
			END`,
		},
		{
			"documents_ad",
			`CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
				DELETE FROM documents_fts WHERE rowid = old.id;
			END`,
		},
		{
			"documents_au",
			`CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents
			BEGIN
				DELETE FROM documents_fts WHERE rowid = old.id AND new.active = 0;
				INSERT OR REPLACE INTO documents_fts(rowid, filepath, title, body)
				SELECT
					new.id,
					new.collection || '/' || new.path,
					new.title,
					(SELECT doc FROM content WHERE hash = new.hash)
				WHERE new.active = 1;
			END`,
		},
	}

	for _, t := range triggers {
		// Drop first for safety / updates
		s.DB.Exec("DROP TRIGGER IF EXISTS " + t.name)
		if _, err := s.DB.Exec(t.body); err != nil {
			return fmt.Errorf("failed to create trigger %s: %w", t.name, err)
		}
	}

	return nil
}

func HashContent(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return hex.EncodeToString(hasher.Sum(nil))
}

func GetDocid(hash string) string {
	if len(hash) < 6 {
		return hash
	}
	return hash[:6]
}

var titleExtractors = map[string]func(string) string{
	".md": func(content string) string {
		rx := regexp.MustCompile(`(?m)^##?\s+(.+)$`)
		match := rx.FindStringSubmatch(content)
		if len(match) > 1 {
			title := strings.TrimSpace(match[1])
			if title == "📝 Notes" || title == "Notes" {
				rxNext := regexp.MustCompile(`(?m)^##\s+(.+)$`)
				nextMatch := rxNext.FindStringSubmatch(content)
				if len(nextMatch) > 1 {
					return strings.TrimSpace(nextMatch[1])
				}
			}
			return title
		}
		return ""
	},
}

func ExtractTitle(content string, filename string) string {
	ext := filepath.Ext(filename)
	if extractor, ok := titleExtractors[strings.ToLower(ext)]; ok {
		if title := extractor(content); title != "" {
			return title
		}
	}
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, ext)
}

func (s *Store) SyncConfigToDb(cfg *config.CollectionConfig) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing configs
	_, err = tx.Exec("DELETE FROM store_collections")
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO store_collections (name, path, pattern, ignore_patterns, include_by_default, update_command, context)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for name, coll := range cfg.Collections {
		ignoreStr := strings.Join(coll.Ignore, ",")
		incByDefault := 1
		if coll.IncludeByDefault != nil && !*coll.IncludeByDefault {
			incByDefault = 0
		}
		var ctxStr string
		if len(coll.Context) > 0 {
			// Serialize contexts as simple comma-separated key:value or JSON.
			// The config/config.go saves context as map[string]string.
			// Let's serialize as JSON so it's robust.
			// But wait, the original TS code serialized context as JSON string in store_collections.
			// Let's do that!
			// We can use gopkg.in/yaml.v3 to convert to YAML or json, or simply write custom parser.
			// Let's write simple JSON context serialization.
			var parts []string
			for k, v := range coll.Context {
				parts = append(parts, fmt.Sprintf("%q:%q", k, v))
			}
			ctxStr = "{" + strings.Join(parts, ",") + "}"
		}

		_, err = stmt.Exec(name, coll.Path, coll.Pattern, ignoreStr, incByDefault, coll.Update, ctxStr)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) RebuildDocumentFTS(documentId int) error {
	var collection, path, title, body string
	err := s.DB.QueryRow(`
		SELECT d.collection, d.path, d.title, content.doc
		FROM documents d
		JOIN content ON content.hash = d.hash
		WHERE d.id = ? AND d.active = 1
	`, documentId).Scan(&collection, &path, &title, &body)

	if err != nil {
		if err == sql.ErrNoRows {
			s.DB.Exec("DELETE FROM documents_fts WHERE rowid = ?", documentId)
			return nil
		}
		return err
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM documents_fts WHERE rowid = ?", documentId)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO documents_fts(rowid, filepath, title, body)
		VALUES (?, ?, ?, ?)
	`,
		documentId,
		NormalizeCjkForFTS(collection+"/"+path),
		NormalizeCjkForFTS(title),
		NormalizeCjkForFTS(body),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ReindexCollection(
	collectionPath string,
	globPattern string,
	collectionName string,
	ignorePatterns []string,
	onProgress func(info ReindexProgress),
) (*ReindexResult, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	files, err := scanFiles(collectionPath, globPattern, ignorePatterns)
	if err != nil {
		return nil, err
	}

	total := len(files)
	res := &ReindexResult{}
	seenPaths := make(map[string]bool)

	for idx, relFile := range files {
		seenPaths[relFile] = true
		fullPath := filepath.Join(collectionPath, relFile)

		data, err := os.ReadFile(fullPath)
		if err != nil {
			if onProgress != nil {
				onProgress(ReindexProgress{File: relFile, Current: idx + 1, Total: total})
			}
			continue
		}

		content := string(data)
		if strings.TrimSpace(content) == "" {
			if onProgress != nil {
				onProgress(ReindexProgress{File: relFile, Current: idx + 1, Total: total})
			}
			continue
		}

		hash := HashContent(content)
		title := ExtractTitle(content, relFile)

		// Insert or ignore content
		_, err = s.DB.Exec("INSERT OR IGNORE INTO content (hash, doc, created_at) VALUES (?, ?, ?)", hash, content, now)
		if err != nil {
			return nil, err
		}

		// Find existing
		var existingId int
		var existingHash, existingTitle string
		err = s.DB.QueryRow(`
			SELECT id, hash, title FROM documents
			WHERE collection = ? AND path = ? AND active = 1
		`, collectionName, relFile).Scan(&existingId, &existingHash, &existingTitle)

		if err == sql.ErrNoRows {
			// Try case-insensitive fallback or migrated
			err = s.DB.QueryRow(`
				SELECT id, hash, title FROM documents
				WHERE collection = ? AND path COLLATE NOCASE = ? AND active = 1
				ORDER BY id LIMIT 1
			`, collectionName, relFile).Scan(&existingId, &existingHash, &existingTitle)
		}

		stat, _ := os.Stat(fullPath)
		mtime := now
		if stat != nil {
			mtime = stat.ModTime().UTC().Format(time.RFC3339)
		}

		if err == nil {
			// Existing document
			if existingHash == hash {
				if existingTitle != title {
					_, err = s.DB.Exec("UPDATE documents SET title = ?, modified_at = ? WHERE id = ?", title, now, existingId)
					if err != nil {
						return nil, err
					}
					s.RebuildDocumentFTS(existingId)
					res.Updated++
				} else {
					res.Unchanged++
				}
			} else {
				_, err = s.DB.Exec("UPDATE documents SET title = ?, hash = ?, modified_at = ? WHERE id = ?", title, hash, mtime, existingId)
				if err != nil {
					return nil, err
				}
				s.RebuildDocumentFTS(existingId)
				res.Updated++
			}
		} else {
			// New document
			res.Indexed++
			var lastId int64
			result, err := s.DB.Exec(`
				INSERT INTO documents (collection, path, title, hash, created_at, modified_at, active)
				VALUES (?, ?, ?, ?, ?, ?, 1)
				ON CONFLICT(collection, path) DO UPDATE SET
					title = excluded.title,
					hash = excluded.hash,
					modified_at = excluded.modified_at,
					active = 1
			`, collectionName, relFile, title, hash, mtime, mtime)
			if err != nil {
				return nil, err
			}
			lastId, _ = result.LastInsertId()
			if lastId == 0 {
				s.DB.QueryRow("SELECT id FROM documents WHERE collection = ? AND path = ?", collectionName, relFile).Scan(&lastId)
			}
			s.RebuildDocumentFTS(int(lastId))
		}

		if onProgress != nil {
			onProgress(ReindexProgress{File: relFile, Current: idx + 1, Total: total})
		}
	}

	// Deactivate stale docs
	rows, err := s.DB.Query("SELECT path FROM documents WHERE collection = ? AND active = 1", collectionName)
	if err == nil {
		defer rows.Close()
		var toDeactivate []string
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err == nil {
				if !seenPaths[p] {
					toDeactivate = append(toDeactivate, p)
				}
			}
		}
		for _, p := range toDeactivate {
			_, err = s.DB.Exec("UPDATE documents SET active = 0 WHERE collection = ? AND path = ?", collectionName, p)
			if err == nil {
				res.Removed++
				// Delete from FTS via id
				var docId int
				s.DB.QueryRow("SELECT id FROM documents WHERE collection = ? AND path = ?", collectionName, p).Scan(&docId)
				s.DB.Exec("DELETE FROM documents_fts WHERE rowid = ?", docId)
			}
		}
	}

	res.OrphanedCleaned = s.CleanupOrphanedContent()

	return res, nil
}

func (s *Store) SearchFTS(query string, limit int, collectionName string) ([]SearchResult, error) {
	ftsQuery := BuildFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	ftsLimit := limit
	if collectionName != "" {
		ftsLimit = limit * 10
	}

	sqlStr := fmt.Sprintf(`
		WITH fts_matches AS (
			SELECT rowid, bm25(documents_fts, 1.5, 4.0, 1.0) as bm25_score
			FROM documents_fts
			WHERE documents_fts MATCH ?
			ORDER BY bm25_score ASC
			LIMIT %d
		)
		SELECT
			'qmd://' || d.collection || '/' || d.path as filepath,
			d.collection || '/' || d.path as display_path,
			d.title,
			content.doc as body,
			d.hash,
			fm.bm25_score,
			d.collection
		FROM fts_matches fm
		JOIN documents d ON d.id = fm.rowid
		JOIN content ON content.hash = d.hash
		WHERE d.active = 1
	`, ftsLimit)

	var params []interface{}
	params = append(params, ftsQuery)

	if collectionName != "" {
		sqlStr += " AND d.collection = ?"
		params = append(params, collectionName)
	}

	sqlStr += " ORDER BY fm.bm25_score ASC LIMIT ?"
	params = append(params, limit)

	rows, err := s.DB.Query(sqlStr, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var filepathStr, displayPath, title, body, hash, coll string
		var bm25Score float64
		err := rows.Scan(&filepathStr, &displayPath, &title, &body, &hash, &bm25Score, &coll)
		if err != nil {
			return nil, err
		}

		// Convert bm25 (negative, lower is better) into [0..1)
		absScore := bm25Score
		if absScore < 0 {
			absScore = -absScore
		}
		score := absScore / (1.0 + absScore)

		results = append(results, SearchResult{
			Filepath:       filepathStr,
			DisplayPath:    displayPath,
			Title:          title,
			Hash:           hash,
			Docid:          GetDocid(hash),
			CollectionName: coll,
			BodyLength:     len(body),
			Body:           body,
			Context:        s.GetContextForFile(filepathStr),
			Score:          score,
			Source:         "fts",
		})
	}

	return results, nil
}

func (s *Store) GetContextForFile(filepathStr string) string {
	if filepathStr == "" {
		return ""
	}

	var collectionName string
	var relativePath string

	if strings.HasPrefix(filepathStr, "qmd://") {
		// qmd://collection/path
		parsed := strings.TrimPrefix(filepathStr, "qmd://")
		parts := strings.SplitN(parsed, "/", 2)
		if len(parts) == 2 {
			collectionName = parts[0]
			relativePath = parts[1]
		}
	} else {
		// Filesystem path: find matching collection
		var colls []struct {
			name string
			path string
		}
		rows, err := s.DB.Query("SELECT name, path FROM store_collections")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var n, p string
				if err := rows.Scan(&n, &p); err == nil {
					colls = append(colls, struct {
						name string
						path string
					}{name: n, path: p})
				}
			}
		}

		for _, coll := range colls {
			if strings.HasPrefix(filepathStr, coll.path+"/") || filepathStr == coll.path {
				collectionName = coll.name
				if filepathStr == coll.path {
					relativePath = ""
				} else {
					relativePath = strings.TrimPrefix(filepathStr, coll.path+"/")
				}
				break
			}
		}
	}

	if collectionName == "" {
		return ""
	}

	// Verify doc exists
	var active int
	s.DB.QueryRow("SELECT active FROM documents WHERE collection = ? AND path = ?", collectionName, relativePath).Scan(&active)
	if active != 1 {
		return ""
	}

	// Find best matching contexts
	var collContext string
	s.DB.QueryRow("SELECT context FROM store_collections WHERE name = ?", collectionName).Scan(&collContext)

	var globalCtx string
	s.DB.QueryRow("SELECT value FROM store_config WHERE key = 'global_context'").Scan(&globalCtx)

	var contexts []string
	if globalCtx != "" {
		contexts = append(contexts, globalCtx)
	}

	// Context map is stored as a simple JSON string in store_collections
	// e.g. {"/":"Root context","/sub":"Subfolder context"}
	// We parse it manually to avoid importing a JSON parser
	if collContext != "" {
		// Quick manual parse of {"key":"value",...}
		rx := regexp.MustCompile(`"([^"]+)":"([^"]+)"`)
		matches := rx.FindAllStringSubmatch(collContext, -1)
		
		var pathMatches []struct {
			prefix  string
			context string
		}
		for _, m := range matches {
			if len(m) == 3 {
				prefix := m[1]
				ctx := m[2]
				
				normPath := relativePath
				if !strings.HasPrefix(normPath, "/") {
					normPath = "/" + normPath
				}
				normPrefix := prefix
				if !strings.HasPrefix(normPrefix, "/") {
					normPrefix = "/" + normPrefix
				}

				if strings.HasPrefix(normPath, normPrefix) {
					pathMatches = append(pathMatches, struct {
						prefix  string
						context string
					}{prefix: normPrefix, context: ctx})
				}
			}
		}

		if len(pathMatches) > 0 {
			sort.Slice(pathMatches, func(i, j int) bool {
				return len(pathMatches[i].prefix) < len(pathMatches[j].prefix)
			})
			for _, pm := range pathMatches {
				contexts = append(contexts, pm.context)
			}
		}
	}

	return strings.Join(contexts, "\n\n")
}

func (s *Store) FindDocument(collectionName, path string) (int, string, string, error) {
	var id int
	var hash, title string
	err := s.DB.QueryRow(`
		SELECT id, hash, title FROM documents
		WHERE collection = ? AND path = ? AND active = 1
	`, collectionName, path).Scan(&id, &hash, &title)
	return id, hash, title, err
}

func (s *Store) GetDocumentBody(hash string) (string, error) {
	var doc string
	err := s.DB.QueryRow("SELECT doc FROM content WHERE hash = ?", hash).Scan(&doc)
	return doc, err
}

func (s *Store) Vacuum() {
	s.DB.Exec("VACUUM")
}

func (s *Store) CleanupOrphanedContent() int {
	res, err := s.DB.Exec(`
		DELETE FROM content
		WHERE hash NOT IN (SELECT DISTINCT hash FROM documents)
	`)
	if err != nil {
		return 0
	}
	cnt, _ := res.RowsAffected()
	return int(cnt)
}

type CollectionInfo struct {
	Name             string `json:"name"`
	Pwd              string `json:"pwd"`
	GlobPattern      string `json:"glob_pattern"`
	DocCount         int    `json:"doc_count"`
	ActiveCount      int    `json:"active_count"`
	LastModified     string `json:"last_modified"`
	IncludeByDefault bool   `json:"includeByDefault"`
}

func (s *Store) ListCollections() ([]CollectionInfo, error) {
	rows, err := s.DB.Query("SELECT name, path, pattern, include_by_default FROM store_collections")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var colls []CollectionInfo
	for rows.Next() {
		var name, path, pattern string
		var includeByDefault int
		if err := rows.Scan(&name, &path, &pattern, &includeByDefault); err != nil {
			return nil, err
		}

		// Get stats from DB
		var docCount, activeCount int
		var lastModified sql.NullString
		err := s.DB.QueryRow(`
			SELECT
				COUNT(id) as doc_count,
				SUM(CASE WHEN active = 1 THEN 1 ELSE 0 END) as active_count,
				MAX(modified_at) as last_modified
			FROM documents
			WHERE collection = ?
		`, name).Scan(&docCount, &activeCount, &lastModified)

		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		lastModStr := ""
		if lastModified.Valid {
			lastModStr = lastModified.String
		}

		colls = append(colls, CollectionInfo{
			Name:             name,
			Pwd:              path,
			GlobPattern:      pattern,
			DocCount:         docCount,
			ActiveCount:      activeCount,
			LastModified:     lastModStr,
			IncludeByDefault: includeByDefault != 0,
		})
	}
	return colls, nil
}

func (s *Store) RemoveCollection(collectionName string) (int, int, error) {
	res, err := s.DB.Exec("DELETE FROM documents WHERE collection = ?", collectionName)
	if err != nil {
		return 0, 0, err
	}
	deletedDocs, _ := res.RowsAffected()

	cleanedHashes := s.CleanupOrphanedContent()

	_, err = s.DB.Exec("DELETE FROM store_collections WHERE name = ?", collectionName)
	if err != nil {
		return 0, 0, err
	}

	return int(deletedDocs), cleanedHashes, nil
}

func (s *Store) DeleteInactiveDocuments() int {
	res, err := s.DB.Exec("DELETE FROM documents WHERE active = 0")
	if err != nil {
		return 0
	}
	cnt, _ := res.RowsAffected()
	return int(cnt)
}

func scanFiles(collectionPath, globPattern string, ignorePatterns []string) ([]string, error) {
	var files []string

	excludeDirs := []string{"node_modules", ".git", ".cache", "vendor", "dist", "build"}
	var compiledIgnores []string
	for _, d := range excludeDirs {
		compiledIgnores = append(compiledIgnores, "**/"+d+"/**")
	}
	compiledIgnores = append(compiledIgnores, ignorePatterns...)

	err := filepath.Walk(collectionPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		rel, err := filepath.Rel(collectionPath, path)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}

		rel = strings.ReplaceAll(rel, "\\", "/")

		parts := strings.Split(rel, "/")
		for _, part := range parts {
			if strings.HasPrefix(part, ".") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			for _, pat := range compiledIgnores {
				if MatchPath(pat, rel+"/") {
					return filepath.SkipDir
				}
			}
			return nil
		}

		for _, pat := range compiledIgnores {
			if MatchPath(pat, rel) {
				return nil
			}
		}

		if MatchPath(globPattern, rel) {
			files = append(files, rel)
		}

		return nil
	})

	return files, err
}
