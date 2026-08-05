# QMD - Query Markup Documents (Go Port)

Core offline FTS search engine ported from TypeScript to Go. Stripped of LLM, vector search, and MCP integrations.

## Commands

```sh
qmd collection add <path> [--name <n>] [--mask <pattern>]   # Create/index collection
qmd collection list                                         # List all collections
qmd collection remove <name>                                # Remove a collection
qmd collection rename <old> <new>                           # Rename a collection
qmd init                                                    # Create a project-local .qmd index
qmd ls [collection[/path]]                                  # List collections or files in a collection
qmd context add [path] "text"                               # Add context for path (defaults to current dir)
qmd context list                                            # List all contexts
qmd context rm <path>                                       # Remove context
qmd get <file>                                              # Get document content
qmd status                                                  # Show index status and collections
qmd doctor                                                  # Check system health
qmd update                                                  # Re-index collections
qmd search <query>                                          # Full-text keyword search (BM25)
qmd cleanup                                                 # Clean database and vacuum
```

## Options

```sh
# Search & retrieval
-c, --collection <name>  # Restrict search to a collection
-n, --limit <num>        # Number of results (default 20)
--all                    # Return all matches (limit = 10000)
--min-score <num>        # Minimum score threshold
--full                   # Show full document content
--line-numbers           # Enable line numbers in output snippet/body

# Output formats (search, get)
--json                   # Format output as JSON
--csv                    # Format output as CSV
--xml                    # Format output as XML
--md                     # Format output as Markdown
--files                  # Format output as flat file paths
```

## Build and Run

Compile the `qmd` CLI binary:

```sh
make build  # equivalent to: go build -tags "sqlite_fts5" -mod=vendor -o qmd ./cmd/qmd
```

## Tests

Run all unit and integration tests:

```sh
make test   # equivalent to: go test -tags "sqlite_fts5" -mod=vendor -v ./...
```

## Architecture

- SQLite FTS5 for full-text search (BM25) using porter tokenizers.
- Offline-first configuration and sqlite engine.
- Smart chunking: Regex-based breaks, markdown heading/rule priorities, code fence protection.

## Agent Guidelines

- **File Searching**: Always use the `./qmd search` binary command inside the workspace directory (e.g. via `run_command`) instead of the `grep_search` or `grep` tools when looking for content within files.
