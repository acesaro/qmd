package mcp

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/acesaro/qmd/store"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func sendError(w io.Writer, id interface{}, code int, message string, data interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	enc := json.NewEncoder(w)
	enc.Encode(resp)
}

func buildInstructions(s *store.Store) string {
	var totalDocs int
	s.DB.QueryRow("SELECT COUNT(*) FROM documents WHERE active = 1").Scan(&totalDocs)

	var globalCtx string
	s.DB.QueryRow("SELECT value FROM store_config WHERE key = 'global_context'").Scan(&globalCtx)

	var lines []string
	lines = append(lines, fmt.Sprintf("QMD is your local search engine over %d markdown documents.", totalDocs))
	if globalCtx != "" {
		lines = append(lines, fmt.Sprintf("Context: %s", globalCtx))
	}

	colls, _ := s.ListCollections()
	if len(colls) > 0 {
		lines = append(lines, "")
		var names []string
		for _, col := range colls {
			names = append(names, col.Name)
		}
		lines = append(lines, fmt.Sprintf("Collections (scope with `collections` parameter): %s", strings.Join(names, ", ")))
		lines = append(lines, "Call the `status` tool for collection descriptions, paths, and per-collection doc counts.")
	}

	lines = append(lines, "")
	lines = append(lines, "Search: Use `query` with sub-queries:")
	lines = append(lines, "  Always provide `intent` on every search call to disambiguate and improve snippets.")
	lines = append(lines, "")
	lines = append(lines, "Retrieval:")
	lines = append(lines, "  - `get` — single document by path or docid (#abc123). Supports a line-range suffix: `file.md:100` (from line 100) or `file.md:100:40` (40 lines from line 100).")
	lines = append(lines, "  - `multi_get` — batch retrieve by glob (`journals/2025-05*.md`) or comma-separated list.")
	lines = append(lines, "")
	lines = append(lines, "Tips:")
	lines = append(lines, "  - File paths in results are relative to their collection.")
	lines = append(lines, "  - Use `minScore: 0.5` to filter low-confidence results.")
	lines = append(lines, "  - Results include a `context` field describing the content type.")

	return strings.Join(lines, "\n")
}

func listTools() interface{} {
	return map[string]interface{}{
		"tools": []interface{}{
			map[string]interface{}{
				"name":        "query",
				"description": "Search the knowledge base using a query document — one or more typed sub-queries combined for best recall.\n\nEach result includes a `line` field with the absolute 1-indexed line of the best match in the source markdown. To read more context around a hit, call `get(file, fromLine = max(1, line - 20), maxLines = 80, lineNumbers = true)`.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Plain-text query, auto-expanded by the SDK into FTS5 terms, fused and reranked. Recommended default for most searches.",
						},
						"searches": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"type":  map[string]interface{}{"type": "string", "enum": []string{"lex", "vec", "hyde"}},
									"query": map[string]interface{}{"type": "string"},
								},
								"required": []string{"type", "query"},
							},
							"description": "Typed sub-queries to execute.",
						},
						"limit":       map[string]interface{}{"type": "number", "default": 10},
						"minScore":    map[string]interface{}{"type": "number", "default": 0},
						"collections": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Filter to collections (OR match)"},
						"intent":      map[string]interface{}{"type": "string", "description": "Background context to disambiguate the query."},
						"chunkStrategy": map[string]interface{}{"type": "string", "enum": []string{"auto", "regex"}, "default": "regex", "description": "Chunk strategy to use. Set to 'auto' to enable AST-aware code chunking."},
					},
				},
			},
			map[string]interface{}{
				"name":        "get",
				"description": "Retrieve the full content of a document by its file path or docid. Use paths or docids (#abc123) from search results. Suggests similar files if not found.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file":        map[string]interface{}{"type": "string", "description": "File path or docid from search results. Supports line suffix like :100 or :100:40."},
						"fromLine":    map[string]interface{}{"type": "number", "description": "Start from this line number (1-indexed)"},
						"maxLines":    map[string]interface{}{"type": "number", "description": "Maximum number of lines to return"},
						"lineNumbers": map[string]interface{}{"type": "boolean", "default": true, "description": "Add line numbers to output"},
					},
					"required": []string{"file"},
				},
			},
			map[string]interface{}{
				"name":        "multi_get",
				"description": "Retrieve multiple documents by glob pattern or comma-separated list.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pattern":     map[string]interface{}{"type": "string", "description": "Glob pattern or comma-separated list of file paths"},
						"maxLines":    map[string]interface{}{"type": "number", "description": "Maximum lines per file"},
						"maxBytes":    map[string]interface{}{"type": "number", "default": 10240, "description": "Skip files larger than this"},
						"lineNumbers": map[string]interface{}{"type": "boolean", "default": true, "description": "Add line numbers to output"},
					},
					"required": []string{"pattern"},
				},
			},
			map[string]interface{}{
				"name":        "status",
				"description": "Show the status of the QMD index: collections, document counts, and health information.",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}
}

type QueryParams struct {
	Query       *string     `json:"query"`
	Searches    []SubSearch `json:"searches"`
	Limit       int         `json:"limit"`
	MinScore    float64     `json:"minScore"`
	Collections   []string    `json:"collections"`
	Intent        string      `json:"intent"`
	ChunkStrategy string      `json:"chunkStrategy"`
}

type SubSearch struct {
	Type  string `json:"type"`
	Query string `json:"query"`
}

type GetParams struct {
	File        string `json:"file"`
	FromLine    *int   `json:"fromLine"`
	MaxLines    *int   `json:"maxLines"`
	LineNumbers *bool  `json:"lineNumbers"`
}

type MultiGetParams struct {
	Pattern     string `json:"pattern"`
	MaxLines    *int   `json:"maxLines"`
	MaxBytes    int    `json:"maxBytes"`
	LineNumbers *bool  `json:"lineNumbers"`
}

var rangeRegex = regexp.MustCompile(`:(\d+):(\d+)$`)
var colonRegex = regexp.MustCompile(`:(\d+)$`)

func parseFileRange(file string) (string, int, int) {
	fromLine := 0
	maxLines := 0

	if m := rangeRegex.FindStringSubmatch(file); m != nil {
		fromLine, _ = strconv.Atoi(m[1])
		maxLines, _ = strconv.Atoi(m[2])
		file = strings.TrimSuffix(file, m[0])
	} else if m := colonRegex.FindStringSubmatch(file); m != nil {
		fromLine, _ = strconv.Atoi(m[1])
		file = strings.TrimSuffix(file, m[0])
	}

	return file, fromLine, maxLines
}

func sliceBody(body string, fromLine, maxLines int) string {
	if fromLine <= 0 && maxLines <= 0 {
		return body
	}

	lines := strings.Split(body, "\n")
	start := 0
	if fromLine > 0 {
		start = fromLine - 1
		if start < 0 {
			start = 0
		}
		if start >= len(lines) {
			return ""
		}
	}

	end := len(lines)
	if maxLines > 0 {
		end = start + maxLines
		if end > len(lines) {
			end = len(lines)
		}
	}

	return strings.Join(lines[start:end], "\n")
}

func handleRequest(s *store.Store, req *JSONRPCRequest) *JSONRPCResponse {
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{},
				"resources": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "qmd",
				"version": "1.0.0",
			},
			"instructions": buildInstructions(s),
		}

	case "tools/list":
		resp.Result = listTools()

	case "tools/call":
		var callParams struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "Invalid params"}
			return resp
		}

		switch callParams.Name {
		case "query":
			var qp QueryParams
			qp.Limit = 10 // Default
			json.Unmarshal(callParams.Arguments, &qp)

			var queryStr string
			if qp.Query != nil {
				queryStr = *qp.Query
			} else if len(qp.Searches) > 0 {
				var parts []string
				for _, sub := range qp.Searches {
					parts = append(parts, sub.Query)
				}
				queryStr = strings.Join(parts, " ")
			}

			if strings.TrimSpace(queryStr) == "" {
				resp.Result = map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": "Error: provide either 'query' or 'searches'"},
					},
					"isError": true,
				}
				return resp
			}

			results, err := s.SearchFTSMulti(queryStr, qp.Limit, qp.Collections)
			if err != nil {
				resp.Result = map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": fmt.Sprintf("Error: search failed: %v", err)},
					},
					"isError": true,
				}
				return resp
			}

			type SearchResultItem struct {
				Docid   string  `json:"docid"`
				File    string  `json:"file"`
				Title   string  `json:"title"`
				Score   float64 `json:"score"`
				Context string  `json:"context"`
				Line    int     `json:"line"`
				Snippet string  `json:"snippet"`
			}

			var filtered []SearchResultItem
			for _, r := range results {
				if r.Score >= qp.MinScore {
					snippetInfo := store.ExtractSnippetWithStrategy(r.Body, queryStr, 300, r.DisplayPath, qp.ChunkStrategy)
					snippetText := store.AddLineNumbers(snippetInfo.Snippet, snippetInfo.Line)

					filtered = append(filtered, SearchResultItem{
						Docid:   "#" + r.Docid,
						File:    r.DisplayPath,
						Title:   r.Title,
						Score:   mathRound(r.Score * 100) / 100,
						Context: r.Context,
						Line:    snippetInfo.Line,
						Snippet: snippetText,
					})
				}
			}

			// Build summary
			var summary string
			if len(filtered) == 0 {
				summary = fmt.Sprintf("No results found for %q", queryStr)
			} else {
				var lines []string
				lines = append(lines, fmt.Sprintf("Found %d result(s) for %q:\n", len(filtered), queryStr))
				for _, r := range filtered {
					lines = append(lines, fmt.Sprintf("%s %.0f%% %s - %s", r.Docid, r.Score*100, r.File, r.Title))
				}
				summary = strings.Join(lines, "\n")
			}

			resp.Result = map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": summary},
				},
				"structuredContent": map[string]interface{}{
					"results": filtered,
				},
			}

		case "get":
			var gp GetParams
			lineNums := true
			gp.LineNumbers = &lineNums
			json.Unmarshal(callParams.Arguments, &gp)

			fileArg, suffixFrom, suffixMax := parseFileRange(gp.File)
			fromLine := suffixFrom
			if gp.FromLine != nil {
				fromLine = *gp.FromLine
			}
			maxLines := suffixMax
			if gp.MaxLines != nil {
				maxLines = *gp.MaxLines
			}

			var collectionName, innerPath string
			var docId int
			var hash, title string
			var findErr error

			if store.IsVirtualPath(fileArg) {
				vp := store.ParseVirtualPath(fileArg)
				if vp != nil {
					collectionName = vp.CollectionName
					innerPath = vp.Path
				}
				docId, hash, title, findErr = s.FindDocument(collectionName, innerPath)
			} else if strings.HasPrefix(fileArg, "#") {
				prefix := strings.TrimPrefix(fileArg, "#")
				findErr = s.DB.QueryRow(`
					SELECT id, hash, title, collection, path FROM documents
					WHERE hash LIKE ? AND active = 1
				`, prefix+"%").Scan(&docId, &hash, &title, &collectionName, &innerPath)
			} else if len(fileArg) == 6 && !strings.Contains(fileArg, "/") {
				findErr = s.DB.QueryRow(`
					SELECT id, hash, title, collection, path FROM documents
					WHERE hash LIKE ? AND active = 1
				`, fileArg+"%").Scan(&docId, &hash, &title, &collectionName, &innerPath)
			} else {
				// Raw path mapping
				colls, _ := s.ListCollections()
				longestPrefix := ""
				for _, col := range colls {
					if strings.HasPrefix(fileArg, col.Pwd) && len(col.Pwd) > len(longestPrefix) {
						longestPrefix = col.Pwd
						collectionName = col.Name
						innerPath = strings.TrimPrefix(fileArg, col.Pwd)
					}
				}
				if collectionName == "" {
					parts := strings.SplitN(fileArg, "/", 2)
					if len(parts) == 2 {
						var exists int
						s.DB.QueryRow("SELECT COUNT(*) FROM store_collections WHERE name = ?", parts[0]).Scan(&exists)
						if exists > 0 {
							collectionName = parts[0]
							innerPath = parts[1]
						}
					}
				}

				if collectionName != "" {
					innerPath = strings.ReplaceAll(innerPath, "\\", "/")
					innerPath = strings.Trim(innerPath, "/")
					docId, hash, title, findErr = s.FindDocument(collectionName, innerPath)
				} else {
					findErr = fmt.Errorf("could not map target")
				}
			}

			if findErr != nil {
				// Try suggesting similar files
				var suggestions []string
				rows, err := s.DB.Query("SELECT collection, path FROM documents WHERE active = 1 LIMIT 5")
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var c, p string
						if rows.Scan(&c, &p) == nil {
							suggestions = append(suggestions, c+"/"+p)
						}
					}
				}

				msg := fmt.Sprintf("Document not found: %s", gp.File)
				if len(suggestions) > 0 {
					var sLines []string
					sLines = append(sLines, msg, "\nDid you mean one of these?")
					for _, sug := range suggestions {
						sLines = append(sLines, fmt.Sprintf("  - %s", sug))
					}
					msg = strings.Join(sLines, "\n")
				}

				resp.Result = map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": msg},
					},
					"isError": true,
				}
				return resp
			}

			body, err := s.GetDocumentBody(hash)
			if err != nil {
				resp.Result = map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": fmt.Sprintf("Error: failed to load document: %v", err)},
					},
					"isError": true,
				}
				return resp
			}

			sliced := sliceBody(body, fromLine, maxLines)
			if gp.LineNumbers == nil || *gp.LineNumbers {
				startLine := fromLine
				if startLine <= 0 {
					startLine = 1
				}
				sliced = store.AddLineNumbers(sliced, startLine)
			}

			contextStr := s.GetContextForFile("qmd://" + collectionName + "/" + innerPath)
			if contextStr != "" {
				sliced = fmt.Sprintf("<!-- Context: %s -->\n\n%s", contextStr, sliced)
			}

			resp.Result = map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "resource",
						"resource": map[string]interface{}{
							"uri":      fmt.Sprintf("qmd://%s/%s", collectionName, innerPath),
							"name":     collectionName + "/" + innerPath,
							"title":    title,
							"mimeType": "text/markdown",
							"text":     sliced,
						},
					},
				},
			}
			_ = docId // keep compiler happy

		case "multi_get":
			var mp MultiGetParams
			lineNums := true
			mp.LineNumbers = &lineNums
			mp.MaxBytes = 10240
			json.Unmarshal(callParams.Arguments, &mp)

			// We fetch active docs and match
			rows, err := s.DB.Query("SELECT collection, path, title, hash FROM documents WHERE active = 1")
			if err != nil {
				resp.Result = map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": fmt.Sprintf("Error querying DB: %v", err)},
					},
					"isError": true,
				}
				return resp
			}
			defer rows.Close()

			var matchedDocs []struct {
				collection  string
				path        string
				title       string
				hash        string
				displayPath string
			}

			targets := strings.Split(mp.Pattern, ",")
			for i, t := range targets {
				targets[i] = strings.TrimSpace(t)
			}

			for rows.Next() {
				var col, path, title, hash string
				if err := rows.Scan(&col, &path, &title, &hash); err == nil {
					displayPath := col + "/" + path
					matched := false
					for _, target := range targets {
						if store.MatchPath(target, displayPath) || store.MatchPath(target, path) || target == displayPath || target == path {
							matched = true
							break
						}
					}
					if matched {
						matchedDocs = append(matchedDocs, struct {
							collection  string
							path        string
							title       string
							hash        string
							displayPath string
						}{col, path, title, hash, displayPath})
					}
				}
			}

			if len(matchedDocs) == 0 {
				resp.Result = map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": fmt.Sprintf("No files matched pattern: %s", mp.Pattern)},
					},
					"isError": true,
				}
				return resp
			}

			var contentItems []interface{}
			for _, md := range matchedDocs {
				body, err := s.GetDocumentBody(md.hash)
				if err != nil {
					contentItems = append(contentItems, map[string]interface{}{
						"type": "text",
						"text": fmt.Sprintf("Error loading %s: %v", md.displayPath, err),
					})
					continue
				}

				if len(body) > mp.MaxBytes {
					contentItems = append(contentItems, map[string]interface{}{
						"type": "text",
						"text": fmt.Sprintf("[SKIPPED: %s - too large (%d bytes). Use 'get' to retrieve.]", md.displayPath, len(body)),
					})
					continue
				}

				sliced := body
				if mp.MaxLines != nil {
					sliced = sliceBody(body, 1, *mp.MaxLines)
				}

				if mp.LineNumbers == nil || *mp.LineNumbers {
					sliced = store.AddLineNumbers(sliced, 1)
				}

				contextStr := s.GetContextForFile("qmd://" + md.collection + "/" + md.path)
				if contextStr != "" {
					sliced = fmt.Sprintf("<!-- Context: %s -->\n\n%s", contextStr, sliced)
				}

				contentItems = append(contentItems, map[string]interface{}{
					"type": "resource",
					"resource": map[string]interface{}{
						"uri":      fmt.Sprintf("qmd://%s/%s", md.collection, md.path),
						"name":     md.displayPath,
						"title":    md.title,
						"mimeType": "text/markdown",
						"text":     sliced,
					},
				})
			}

			resp.Result = map[string]interface{}{
				"content": contentItems,
			}

		case "status":
			var totalDocs int
			s.DB.QueryRow("SELECT COUNT(*) FROM documents WHERE active = 1").Scan(&totalDocs)

			colls, _ := s.ListCollections()
			type CollStatus struct {
				Name        string `json:"name"`
				Path        string `json:"path"`
				Pattern     string `json:"pattern"`
				Documents   int    `json:"documents"`
				LastUpdated string `json:"lastUpdated"`
			}
			var list []CollStatus
			for _, col := range colls {
				list = append(list, CollStatus{
					Name:        col.Name,
					Path:        col.Pwd,
					Pattern:     col.GlobPattern,
					Documents:   col.ActiveCount,
					LastUpdated: col.LastModified,
				})
			}

			summary := []string{
				"QMD Index Status:",
				fmt.Sprintf("  Total documents: %d", totalDocs),
				fmt.Sprintf("  Needs embedding: 0"),
				fmt.Sprintf("  Vector index: no"),
				fmt.Sprintf("  Collections: %d", len(list)),
			}
			for _, c := range list {
				summary = append(summary, fmt.Sprintf("    - %s: %s (%d docs)", c.Name, c.Path, c.Documents))
			}

			resp.Result = map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": strings.Join(summary, "\n")},
				},
				"structuredContent": map[string]interface{}{
					"totalDocuments": totalDocs,
					"needsEmbedding": 0,
					"hasVectorIndex": false,
					"collections":    list,
				},
			}

		default:
			resp.Error = &RPCError{Code: -32601, Message: "Method not found"}
		}

	case "notifications/initialized":
		// Notification, do not respond
		return nil

	default:
		resp.Error = &RPCError{Code: -32601, Message: "Method not found"}
	}

	return resp
}

func mathRound(f float64) float64 {
	return mathRoundToGo(f)
}

func mathRoundToGo(f float64) float64 {
	// Standard round
	if f < 0 {
		return float64(int(f - 0.5))
	}
	return float64(int(f + 0.5))
}

func StartStdioServer(s *store.Store) error {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(os.Stdout, nil, -32700, "Parse error", nil)
			continue
		}

		resp := handleRequest(s, &req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			os.Stdout.Write(data)
			os.Stdout.Write([]byte("\n"))
		}
	}
	return scanner.Err()
}

func StartHttpServer(s *store.Store, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	fmt.Fprintf(os.Stderr, "QMD MCP server listening on http://localhost:%d/mcp\n", actualPort)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Generate static/random session ID
	sessionBytes := make([]byte, 8)
	rand.Read(sessionBytes)
	sessionID := fmt.Sprintf("session-%x", sessionBytes)

	http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Handle session routing
		reqSessionID := r.Header.Get("Mcp-Session-Id")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(body, &req); err != nil {
			sendError(w, nil, -32700, "Parse error", nil)
			return
		}

		if reqSessionID == "" && req.Method != "initialize" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32000,"message":"Bad Request: Missing session ID"},"id":null}`))
			return
		}

		resp := handleRequest(s, &req)
		if resp != nil {
			if req.Method == "initialize" {
				w.Header().Set("Mcp-Session-Id", sessionID)
			}
			enc := json.NewEncoder(w)
			enc.Encode(resp)
		}
	})

	// Also support REST /query and /search endpoints
	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		handleRestSearch(s, w, r)
	})
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		handleRestSearch(s, w, r)
	})

	server := &http.Server{}
	return server.Serve(listener)
}

func handleRestSearch(s *store.Store, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var params struct {
		Searches []struct {
			Type  string `json:"type"`
			Query string `json:"query"`
		} `json:"searches"`
		Collections   []string `json:"collections"`
		Limit         int      `json:"limit"`
		MinScore      float64  `json:"minScore"`
		Intent        string   `json:"intent"`
		ChunkStrategy string   `json:"chunkStrategy"`
	}
	params.Limit = 10
	if err := json.Unmarshal(body, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Invalid params"}`))
		return
	}

	if len(params.Searches) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Missing required field: searches"}`))
		return
	}

	var queryParts []string
	for _, search := range params.Searches {
		queryParts = append(queryParts, search.Query)
	}
	queryStr := strings.Join(queryParts, " ")

	results, err := s.SearchFTSMulti(queryStr, params.Limit, params.Collections)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"error":"%v"}`, err)))
		return
	}

	type FormattedItem struct {
		Docid   string  `json:"docid"`
		File    string  `json:"file"`
		Title   string  `json:"title"`
		Score   float64 `json:"score"`
		Context string  `json:"context"`
		Line    int     `json:"line"`
		Snippet string  `json:"snippet"`
	}

	var formatted []FormattedItem
	for _, r := range results {
		if r.Score >= params.MinScore {
			snippetInfo := store.ExtractSnippetWithStrategy(r.Body, queryStr, 300, r.DisplayPath, params.ChunkStrategy)
			snippetText := store.AddLineNumbers(snippetInfo.Snippet, snippetInfo.Line)

			formatted = append(formatted, FormattedItem{
				Docid:   "#" + r.Docid,
				File:    "qmd://" + r.DisplayPath,
				Title:   r.Title,
				Score:   mathRound(r.Score * 100) / 100,
				Context: r.Context,
				Line:    snippetInfo.Line,
				Snippet: snippetText,
			})
		}
	}

	enc := json.NewEncoder(w)
	enc.Encode(map[string]interface{}{"results": formatted})
}
