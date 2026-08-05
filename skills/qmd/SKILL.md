---
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

1. Search for candidate documents using `qmd search`.
2. Retrieve the full source with `qmd get` or `qmd multi-get`.
3. Answer from retrieved text, citing paths or docids.

Do not answer from snippets alone when the user needs facts, decisions, quotes, or nuance. Snippets are only leads.

Typical loop:

```bash
qmd search "merchant reality support interviews" -n 5
# leads: #abc123 concepts/customer-proximity.md; #def432 sources/merchant-call.md
qmd get "#abc123"
```

When reporting what you retrieved, a compact note is enough; do not paste whole files unless needed:

```text
Retrieved:
- #abc123 concepts/customer-proximity.md
- #def432 sources/merchant-call.md
```

## Pick the right search mode

QMD uses **SQLite FTS5 BM25 keyword search**. It does not run local LLM embedding models. Combine exact anchors, phrases, or logical terms to find relevant files:

```bash
qmd search "cockpit OKR Goodhart" -n 10
qmd search '"AI Before Headcount"' -c concepts -n 5
```

If you are using the MCP server, you can call the `query` tool. If multiple sub-queries are supplied under the `searches` parameter, the server joins them with spaces to perform a unified keyword search.

## Retrieve sources

Search results include docids like `#abc123` and collection-relative paths. Fetch them:

```bash
qmd get "#abc123"
qmd get concepts/ai-before-headcount.md
qmd get "#abc123:120:40"                  # lines 120–159
```

### Output is line-numbered and carries the docid — cite both

`get` is **line-numbered by default** and prints the document's path. Cite the docid and exact line numbers in your answer.

### Read line ranges with the `:from:count` suffix — never pipe through `sed`/`head`/`tail`

QMD slices files itself. Use the suffix or flags; do **not** shell out to `sed`, `head`, or `tail`.

```bash
qmd get "#abc123:120:40"                  # 40 lines starting at line 120
qmd get concepts/note.md:200:60           # lines 200–259
qmd get "#abc123" --from 120 -l 40         # equivalent, using flags
```

## Discover what is indexed

```bash
qmd collection list
qmd ls
qmd status
```

Add collection filters when broad searches drift into the wrong corpus:

```bash
qmd search "headcount autonomous agents" -c concepts -n 10
```

Omit `-c` to search everything.

## MCP Tool: `query`

When using the MCP server, run searches using the `query` tool:

```json
{
  "query": "cockpit OKR Goodhart metrics judgment",
  "collections": ["concepts"],
  "limit": 10
}
```

Note that vector search and query expansion (e.g. HyDE) are not supported. If sub-queries are supplied in `searches`, the server joins them with spaces and performs a unified keyword search.

## Setup and maintenance

Only mutate indexes when the user asked for setup or maintenance.

```bash
# Initialize a local index
qmd init

# Index a folder of markdown files
qmd collection add ~/notes --name notes --mask '**/*.md'

# Update the search index (re-scans collections for changes)
qmd update
```

Health and diagnostics:

```bash
qmd doctor
qmd status
qmd cleanup
```

## Pitfalls

- **Do not stop at snippets.** Fetch documents before making claims.
- **Do not slice files with `sed`/`head`/`tail`.** Use the `path:from:count` suffix or `--from`/`-l`.
- **No vector embeddings or vector-backed query expansion exists in this port.** Focus queries on exact terms, names, titles, and logical keyword structures.
- **Collection names matter.** Search target collections to narrow down results.
