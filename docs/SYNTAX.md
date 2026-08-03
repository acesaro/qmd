# QMD Search Query Syntax

QMD uses SQLite FTS5 for local full-text search (BM25) over indexed documents. 

## Search Syntax

Search queries support special syntax for precise keyword matching and exclusions:

```ebnf
search_query = { search_term } ;
search_term  = negation | phrase | word ;
negation     = "-" ( phrase | word ) ;
phrase       = '"' { character } '"' ;
word         = { letter | digit | "'" } ;
```

| Syntax | Meaning | Example |
|--------|---------|---------|
| `word` | Prefix match | `perf` matches "performance", "perf", etc. |
| `"phrase"` | Exact phrase | `"rate limiter"` matches the exact consecutive sequence of words. |
| `-word` | Exclude term | `-sports` excludes any document containing the term "sports". |
| `-"phrase"` | Exclude phrase | `-"test data"` excludes any document containing the phrase "test data". |

### Examples

```sh
qmd search "CAP theorem consistency"
qmd search '"machine learning" -"deep learning"'
qmd search "auth -oauth -saml"
```

## Scoping Searches

You can restrict your search queries to specific collections with the `-c` or `--collection` flag:

```sh
# Search only in the 'docs' collection
qmd search -c docs "authentication flow"
```

When using the MCP Server, you can pass a `collections` array filter:

```json
{
  "name": "query",
  "arguments": {
    "query": "auth",
    "collections": ["docs", "notes"]
  }
}
```

## Strategy Settings

Search and snippet extraction can be configured to use AST-aware chunking boundaries to select more context-relevant blocks:

- `--chunk-strategy auto`: Uses AST-derived syntactic boundaries (functions, classes, tables, struct scopes) for code files (`.ts`, `.tsx`, `.js`, `.jsx`, `.py`, `.go`, `.rs`, `.sql`, `.lua`) to extract snippet blocks.
- `--chunk-strategy regex`: (Default) Uses paragraph, markdown header, or code fence boundaries.

Example:
```sh
qmd search "Authenticate" --chunk-strategy auto
```
