package mcp

import "strings"

var SkillMd = strings.ReplaceAll(`---
name: qmd
description: Search local markdown knowledge bases, notes, docs, and wikis with QMD. Use when users ask to find notes, retrieve documents, inspect a wiki, answer from indexed markdown, or set up QMD access.
license: MIT
compatibility: Requires qmd CLI or MCP server. Built in Go.
metadata:
  author: acesaro
  version: "3.0.0"
allowed-tools: Bash(qmd:*), mcp__qmd__*
---

# QMD - Query Markdown Documents (Go Port)

## How search works

QMD searches local markdown collections using SQLite FTS5 (BM25 keyword search) over notes, docs, wikis, transcripts, and project knowledge bases. Use it before web search when the answer may already be in indexed local files.

The workflow is always:

1. Search for candidate documents using BACKTICKqmd searchBACKTICK.
2. Retrieve the full source with BACKTICKqmd getBACKTICK or BACKTICKqmd multi-getBACKTICK.
3. Answer from retrieved text, citing paths or docids.

Do not answer from snippets alone when the user needs facts, decisions, quotes, or nuance. Snippets are only leads.

Typical loop:

BACKTICKbash
qmd search "merchant reality support interviews" -n 5
# leads: #abc123 concepts/customer-proximity.md; #def432 sources/merchant-call.md
qmd get "#abc123"
BACKTICK

When reporting what you retrieved, a compact note is enough; do not paste whole files unless needed:

BACKTICKtext
Retrieved:
- #abc123 concepts/customer-proximity.md
- #def432 sources/merchant-call.md
BACKTICK

## Pick the right search mode

QMD uses **SQLite FTS5 BM25 keyword search**. It does not run local LLM embedding models. Combine exact anchors, phrases, or logical terms to find relevant files:

BACKTICKbash
qmd search "cockpit OKR Goodhart" -n 10
qmd search '"AI Before Headcount"' -c concepts -n 5
BACKTICK

If you are using the MCP server, you can call the BACKTICKqueryBACKTICK tool. If multiple sub-queries are supplied under the BACKTICKsearchesBACKTICK parameter, the server joins them with spaces to perform a unified keyword search.

## Retrieve sources

Search results include docids like BACKTICK#abc123BACKTICK and collection-relative paths. Fetch them:

BACKTICKbash
qmd get "#abc123"
qmd get concepts/ai-before-headcount.md
qmd get "#abc123:120:40"                  # lines 120–159
BACKTICK

### Output is line-numbered and carries the docid — cite both

BACKTICKgetBACKTICK is **line-numbered by default** and prints the document's path. Cite the docid and exact line numbers in your answer.

### Read line ranges with the BACKTICK:from:countBACKTICK suffix — never pipe through BACKTICKsedBACKTICK/BACKTICKheadBACKTICK/BACKTICKtailBACKTICK

QMD slices files itself. Use the suffix or flags; do **not** shell out to BACKTICKsedBACKTICK, BACKTICKheadBACKTICK, or BACKTICKtailBACKTICK.

BACKTICKbash
qmd get "#abc123:120:40"                  # 40 lines starting at line 120
qmd get concepts/note.md:200:60           # lines 200–259
qmd get "#abc123" --from 120 -l 40         # equivalent, using flags
BACKTICK

## Discover what is indexed

BACKTICKbash
qmd collection list
qmd ls
qmd status
BACKTICK

Add collection filters when broad searches drift into the wrong corpus:

BACKTICKbash
qmd search "headcount autonomous agents" -c concepts -n 10
BACKTICK

Omit BACKTICK-cBACKTICK to search everything.

## MCP Tool: BACKTICKqueryBACKTICK

When using the MCP server, run searches using the BACKTICKqueryBACKTICK tool:

BACKTICKjson
{
  "query": "cockpit OKR Goodhart metrics judgment",
  "collections": ["concepts"],
  "limit": 10
}
BACKTICK

Note that vector search and query expansion (e.g. HyDE) are not supported. If sub-queries are supplied in BACKTICKsearchesBACKTICK, the server joins them with spaces and performs a unified keyword search.

## Setup and maintenance

Only mutate indexes when the user asked for setup or maintenance.

BACKTICKbash
# Initialize a local index
qmd init

# Index a folder of markdown files
qmd collection add ~/notes --name notes --mask '**/*.md'

# Update the search index (re-scans collections for changes)
qmd update
BACKTICK

Health and diagnostics:

BACKTICKbash
qmd doctor
qmd status
qmd cleanup
BACKTICK

## Pitfalls

- **Do not stop at snippets.** Fetch documents before making claims.
- **Do not slice files with BACKTICKsedBACKTICK/BACKTICKheadBACKTICK/BACKTICKtailBACKTICK.** Use the BACKTICKpath:from:countBACKTICK suffix or BACKTICK--fromBACKTICK/BACKTICK-lBACKTICK.
- **No vector embeddings or vector-backed query expansion exists in this port.** Focus queries on exact terms, names, titles, and logical keyword structures.
- **Collection names matter.** Search target collections to narrow down results.
`, "BACKTICK", "`")

var McpSetupMd = strings.ReplaceAll(`# QMD MCP Server Setup (Go Port)

## Install

Build the Go binary and place it in your system PATH:

BACKTICKbash
go build -tags "sqlite_fts5" -mod=vendor -o qmd ./cmd/qmd
# Put 'qmd' binary in your PATH
qmd init
qmd collection add ~/path/to/markdown --name myknowledge
qmd update
BACKTICK

## Configure MCP Client

**Claude Code** (BACKTICK~/.claude/settings.jsonBACKTICK):
BACKTICKjson
{
  "mcpServers": {
    "qmd": { "command": "qmd", "args": ["mcp"] }
  }
}
BACKTICK

**Claude Desktop** (BACKTICK~/Library/Application Support/Claude/claude_desktop_config.jsonBACKTICK):
BACKTICKjson
{
  "mcpServers": {
    "qmd": { "command": "qmd", "args": ["mcp"] }
  }
}
BACKTICK

**OpenClaw** (BACKTICK~/.openclaw/openclaw.jsonBACKTICK):
BACKTICKjson
{
  "mcp": {
    "servers": {
      "qmd": { "command": "qmd", "args": ["mcp"] }
    }
  }
}
BACKTICK

## HTTP Mode

BACKTICKbash
qmd mcp --http              # Port 8181
BACKTICK

## Tools

### query

Search indexed collections using SQLite FTS5 BM25 search.

BACKTICKjson
{
  "query": "keyword phrases",
  "limit": 10,
  "collections": ["myknowledge"],
  "minScore": 0.0
}
BACKTICK

Note: If BACKTICKsearchesBACKTICK is specified instead of BACKTICKqueryBACKTICK, the server joins the sub-queries with spaces and runs a unified FTS5 search. Vector/semantic queries are not supported.

### get

Retrieve document content by file path or docid, supporting line number ranges.

| Param | Type | Description |
|-------|------|-------------|
| BACKTICKfileBACKTICK | string | File path or docid from search results (supports suffix like :100 or :100:40) |
| BACKTICKfromLineBACKTICK | number? | Start from this line number (1-indexed) |
| BACKTICKmaxLinesBACKTICK | number? | Maximum number of lines to return |
| BACKTICKlineNumbersBACKTICK | boolean? | Add line numbers to output (default true) |

### multi_get

Retrieve multiple documents by glob pattern or comma-separated list.

| Param | Type | Description |
|-------|------|-------------|
| BACKTICKpatternBACKTICK | string | Glob pattern or comma-separated list of file paths |
| BACKTICKmaxLinesBACKTICK | number? | Maximum lines per file |
| BACKTICKmaxBytesBACKTICK | number? | Skip files larger than this (default 10240 bytes) |
| BACKTICKlineNumbersBACKTICK | boolean? | Add line numbers to output (default true) |

### status

Index health and collections. No params.

## Troubleshooting

- **Not starting**: BACKTICKwhich qmdBACKTICK, check PATH, or run BACKTICKqmd mcpBACKTICK manually.
- **No results**: Run BACKTICKqmd collection listBACKTICK and BACKTICKqmd updateBACKTICK.
`, "BACKTICK", "`")
