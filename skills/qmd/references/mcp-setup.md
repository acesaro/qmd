# QMD MCP Server Setup (Go Port)

## Install

Build the Go binary and place it in your system PATH:

```bash
go build -tags "sqlite_fts5" -mod=vendor -o qmd ./cmd/qmd
# Put 'qmd' binary in your PATH
qmd init
qmd collection add ~/path/to/markdown --name myknowledge
qmd update
```

## Configure MCP Client

**Claude Code** (`~/.claude/settings.json`):
```json
{
  "mcpServers": {
    "qmd": { "command": "qmd", "args": ["mcp"] }
  }
}
```

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "qmd": { "command": "qmd", "args": ["mcp"] }
  }
}
```

**OpenClaw** (`~/.openclaw/openclaw.json`):
```json
{
  "mcp": {
    "servers": {
      "qmd": { "command": "qmd", "args": ["mcp"] }
    }
  }
}
```

## HTTP Mode

```bash
qmd mcp --http              # Port 8181
```

## Tools

### query

Search indexed collections using SQLite FTS5 BM25 search.

```json
{
  "query": "keyword phrases",
  "limit": 10,
  "collections": ["myknowledge"],
  "minScore": 0.0
}
```

Note: If `searches` is specified instead of `query`, the server joins the sub-queries with spaces and runs a unified FTS5 search. Vector/semantic queries are not supported.

### get

Retrieve document content by file path or docid, supporting line number ranges.

| Param | Type | Description |
|-------|------|-------------|
| `file` | string | File path or docid from search results (supports suffix like :100 or :100:40) |
| `fromLine` | number? | Start from this line number (1-indexed) |
| `maxLines` | number? | Maximum number of lines to return |
| `lineNumbers` | boolean? | Add line numbers to output (default true) |

### multi_get

Retrieve multiple documents by glob pattern or comma-separated list.

| Param | Type | Description |
|-------|------|-------------|
| `pattern` | string | Glob pattern or comma-separated list of file paths |
| `maxLines` | number? | Maximum lines per file |
| `maxBytes` | number? | Skip files larger than this (default 10240 bytes) |
| `lineNumbers` | boolean? | Add line numbers to output (default true) |

### status

Index health and collections. No params.

## Troubleshooting

- **Not starting**: `which qmd`, check PATH, or run `qmd mcp` manually.
- **No results**: Run `qmd collection list` and `qmd update`.
