# QMD - Query Markup Documents (Go Port)

An on-device search engine for indexing your markdown notes, knowledge bases, and code repository files. QMD uses SQLite FTS5 for fast, local BM25 keyword search, structured folder context tracking, and smart markdown chunking.

## Features

- **SQLite FTS5 BM25 Search**: Native porter/unicode tokenized keyword search.
- **Smart Markdown Chunking**: Regex-based document break scanner prioritizing headings, horizontal rules, and paragraph boundaries without splitting code blocks.
- **Context Trees**: Allows attaching hierarchical description context to folder structures.
- **Offline & Offline-first**: Built directly in Go, with all dependencies vendored locally. Zero network calls.

## Installation

Building from source:

```sh
go build -tags "sqlite_fts5" -mod=vendor -o qmd ./cmd/qmd
```

Put the compiled `qmd` binary in your PATH.

## Quick Start

```sh
# Initialize a local index inside a project directory
qmd init

# Index a folder of markdown files
qmd collection add ~/notes --name notes --mask '**/*.md'

# List indexed collections
qmd collection list

# List files inside a collection
qmd ls notes

# Search for terms using full-text search
qmd search "database replication"

# Search in a specific collection
qmd search "database replication" --collection notes

# Output search results as JSON
qmd search "database replication" --json

# Get document content
qmd get notes/index.md
```

## Options

### Search and Retrieval
- `-c, --collection <name>`: Restrict search to collection
- `-n, --limit <num>`: Number of results (default 20)
- `--all`: Return all matches (limit = 10000)
- `--min-score <num>`: Minimum score threshold
- `--full`: Show full document content
- `--line-numbers`: Enable line numbers in snippets or full body

### Output Formatting
- `--json`: Format output as JSON
- `--csv`: Format output as CSV
- `--xml`: Format output as XML
- `--md`: Format output as Markdown
- `--files`: Format output as flat file paths
