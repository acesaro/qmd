package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func InstallMcpReferences() error {
	// 1. Get executable path of qmd
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get qmd executable path: %v", err)
	}

	// If run via go run, fall back to current directory binary path or absolute path
	if filepath.Base(execPath) == "main" || stringsContains(execPath, "/tmp/go-build") {
		pwd, _ := os.Getwd()
		execPath = filepath.Join(pwd, "qmd")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to find user home directory: %v", err)
	}

	// 2. Install to antigravity CLI directory
	antigravityMcpDir := filepath.Join(homeDir, ".gemini", "antigravity-cli", "mcp", "qmd")
	if err := os.MkdirAll(antigravityMcpDir, 0755); err != nil {
		return fmt.Errorf("failed to create antigravity MCP directory: %v", err)
	}

	schemas := map[string]string{
		"query.json": `{
  "name": "query",
  "description": "Search the knowledge base using a query document — one or more typed sub-queries combined for best recall.\n\nEach result includes a ` + "`" + `line` + "`" + ` field with the absolute 1-indexed line of the best match in the source markdown. To read more context around a hit, call ` + "`" + `get(file, fromLine = max(1, line - 20), maxLines = 80, lineNumbers = true)` + "`" + `.",
  "parameters": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "Plain-text query, auto-expanded by the SDK into FTS5 terms, fused and reranked. Recommended default for most searches."
      },
      "searches": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "type": { "type": "string", "enum": ["lex", "vec", "hyde"] },
            "query": { "type": "string" }
          },
          "required": ["type", "query"]
        },
        "description": "Typed sub-queries to execute."
      },
      "limit": { "type": "number", "default": 10 },
      "minScore": { "type": "number", "default": 0 },
      "collections": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Filter to collections (OR match)"
      },
      "intent": {
        "type": "string",
        "description": "Background context to disambiguate the query."
      },
      "chunkStrategy": {
        "type": "string",
        "enum": ["auto", "regex"],
        "default": "regex",
        "description": "Chunk strategy to use. Set to 'auto' to enable AST-aware code chunking."
      }
    }
  }
}`,
		"get.json": `{
  "name": "get",
  "description": "Retrieve the full content of a document by its file path or docid. Use paths or docids (#abc123) from search results. Suggests similar files if not found.",
  "parameters": {
    "type": "object",
    "properties": {
      "file": { "type": "string", "description": "File path or docid from search results. Supports line suffix like :100 or :100:40." },
      "fromLine": { "type": "number", "description": "Start from this line number (1-indexed)" },
      "maxLines": { "type": "number", "description": "Maximum number of lines to return" },
      "lineNumbers": { "type": "boolean", "default": true, "description": "Add line numbers to output" }
    },
    "required": ["file"]
  }
}`,
		"multi_get.json": `{
  "name": "multi_get",
  "description": "Retrieve multiple documents by glob pattern or comma-separated list.",
  "parameters": {
    "type": "object",
    "properties": {
      "pattern": { "type": "string", "description": "Glob pattern or comma-separated list of file paths" },
      "maxLines": { "type": "number", "description": "Maximum lines per file" },
      "maxBytes": { "type": "number", "default": 10240, "description": "Skip files larger than this" },
      "lineNumbers": { "type": "boolean", "default": true, "description": "Add line numbers to output" }
    },
    "required": ["pattern"]
  }
}`,
		"status.json": `{
  "name": "status",
  "description": "Show the status of the QMD index: collections, document counts, and health information.",
  "parameters": {
    "type": "object",
    "properties": {}
  }
}`,
		"instructions.md": `QMD is your local search engine over indexed documents.
Use query for keyword searches, get to retrieve full content, multi_get to load multiple matches, and status to monitor index health.`,
	}

	for filename, schema := range schemas {
		schemaPath := filepath.Join(antigravityMcpDir, filename)
		if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
			return fmt.Errorf("failed to write %s to antigravity MCP directory: %v", filename, err)
		}
	}
	fmt.Printf("✓ Installed QMD MCP server schemas to antigravity CLI (~/.gemini/antigravity-cli/mcp/qmd/)\n")

	// 3. Configure other GenAI harnesses JSON files
	configs := []string{
		// Claude Desktop
		filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		filepath.Join(homeDir, ".config", "Claude", "claude_desktop_config.json"),
		// Cursor
		filepath.Join(homeDir, "Library", "Application Support", "Cursor", "User", "globalStorage", "moogle.cursor-client", "mcpServers.json"),
		filepath.Join(homeDir, ".config", "Cursor", "User", "globalStorage", "moogle.cursor-client", "mcpServers.json"),
		// Claude Code CLI
		filepath.Join(homeDir, ".claude.json"),
		filepath.Join(homeDir, ".config", "claude", "config.json"),
		// Copilot CLI
		filepath.Join(homeDir, ".config", "github-copilot", "mcp.json"),
		filepath.Join(homeDir, ".config", "github-copilot", "config.json"),
		// pi coding agent
		filepath.Join(homeDir, ".config", "pi", "config.json"),
		filepath.Join(homeDir, ".pi", "config.json"),
	}

	for _, cfgPath := range configs {
		// Only configure if the directory structure for that app exists or if the file exists
		dir := filepath.Dir(cfgPath)
		if _, err := os.Stat(dir); err == nil {
			if err := updateMcpConfig(cfgPath, execPath); err == nil {
				fmt.Printf("✓ Configured QMD MCP server in %s\n", cfgPath)
			}
		}
	}

	return nil
}

func updateMcpConfig(filePath string, execPath string) error {
	data, err := os.ReadFile(filePath)
	var config map[string]interface{}
	if err != nil {
		config = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &config); err != nil {
			config = make(map[string]interface{})
		}
	}

	mcpServersRaw, ok := config["mcpServers"]
	var mcpServers map[string]interface{}
	if ok {
		mcpServers, _ = mcpServersRaw.(map[string]interface{})
	}
	if mcpServers == nil {
		mcpServers = make(map[string]interface{})
	}

	mcpServers["qmd"] = map[string]interface{}{
		"command": execPath,
		"args":    []string{"mcp"},
	}

	config["mcpServers"] = mcpServers

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, out, 0644)
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || stringsIndex(s, substr) >= 0)
}

func stringsIndex(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
