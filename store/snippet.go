package store

import (
	"fmt"
	"strings"
)

type SnippetResult struct {
	Line         int    `json:"line"`
	Snippet      string `json:"snippet"`
	LinesBefore  int    `json:"linesBefore"`
	LinesAfter   int    `json:"linesAfter"`
	SnippetLines int    `json:"snippetLines"`
}

func AddLineNumbers(text string, startLine int) string {
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("%d: %s", startLine+i, line))
	}
	return sb.String()
}

func ExtractSnippet(body string, query string, maxLen int) SnippetResult {
	lines := strings.Split(body, "\n")
	totalLines := len(lines)

	var queryTerms []string
	for _, term := range strings.Fields(strings.ToLower(query)) {
		if term != "" {
			queryTerms = append(queryTerms, term)
		}
	}

	bestLine := 0
	bestScore := -1.0

	for i, line := range lines {
		lineLower := strings.ToLower(line)
		score := 0.0
		for _, term := range queryTerms {
			if strings.Contains(lineLower, term) {
				score += 1.0
			}
		}
		if score > bestScore {
			bestScore = score
			bestLine = i
		}
	}

	start := bestLine - 1
	if start < 0 {
		start = 0
	}
	end := bestLine + 3
	if end > len(lines) {
		end = len(lines)
	}

	snippetLines := lines[start:end]
	snippetText := strings.Join(snippetLines, "\n")

	if len(snippetText) > maxLen {
		if maxLen > 3 {
			snippetText = snippetText[:maxLen-3] + "..."
		} else {
			snippetText = snippetText[:maxLen]
		}
	}

	absoluteStart := start + 1
	snippetLineCount := len(snippetLines)
	linesBefore := absoluteStart - 1
	linesAfter := totalLines - (absoluteStart + snippetLineCount - 1)

	header := fmt.Sprintf("@@ -%d,%d @@ (%d before, %d after)", absoluteStart, snippetLineCount, linesBefore, linesAfter)
	snippet := header + "\n" + snippetText

	return SnippetResult{
		Line:         bestLine + 1,
		Snippet:      snippet,
		LinesBefore:  linesBefore,
		LinesAfter:   linesAfter,
		SnippetLines: snippetLineCount,
	}
}

func ExtractSnippetWithStrategy(body string, query string, maxLen int, filePath string, strategy string) SnippetResult {
	if strategy == "auto" && filePath != "" && detectLanguage(filePath) != LangNone {
		chunks := ChunkDocumentWithStrategy(body, filePath, "auto")
		if len(chunks) > 0 {
			queryTerms := strings.Fields(strings.ToLower(query))
			bestIdx := 0
			bestScore := -1.0
			for i, chunk := range chunks {
				chunkLower := strings.ToLower(chunk.Text)
				score := 0.0
				for _, term := range queryTerms {
					if strings.Contains(chunkLower, term) {
						score += 1.0
					}
				}
				if score > bestScore {
					bestScore = score
					bestIdx = i
				}
			}
			bestChunk := chunks[bestIdx]

			startLine := 1
			if bestChunk.Pos > 0 {
				startLine = len(strings.Split(body[:bestChunk.Pos], "\n"))
			}

			res := ExtractSnippet(bestChunk.Text, query, maxLen)
			res.Line = startLine + res.Line - 1

			totalLines := len(strings.Split(body, "\n"))
			res.LinesBefore = res.Line - 1
			res.LinesAfter = totalLines - (res.Line + res.SnippetLines - 1)

			header := fmt.Sprintf("@@ -%d,%d @@ (%d before, %d after)", res.Line, res.SnippetLines, res.LinesBefore, res.LinesAfter)

			parts := strings.SplitN(res.Snippet, "\n", 2)
			if len(parts) == 2 {
				res.Snippet = header + "\n" + parts[1]
			}

			return res
		}
	}
	return ExtractSnippet(body, query, maxLen)
}
