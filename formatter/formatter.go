package formatter

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"strings"

	"github.com/acesaro/qmd/store"
)

type MultiGetFile struct {
	Filepath    string `json:"filepath"`
	DisplayPath string `json:"displayPath"`
	Title       string `json:"title"`
	Body        string `json:"body,omitempty"`
	Context     string `json:"context,omitempty"`
	Skipped     bool   `json:"skipped"`
	SkipReason  string `json:"skipReason,omitempty"`
}

type FormatOptions struct {
	Full        bool
	Query       string
	UseColor    bool
	LineNumbers bool
}

func EscapeCSV(value string) string {
	if strings.ContainsAny(value, ",\"\n\r") {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return value
}

func EscapeXML(str string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		`'`, "&apos;",
	).Replace(str)
}

func SearchResultsToJson(results []store.SearchResult, opts FormatOptions) string {
	type jsonRow struct {
		Docid      string  `json:"docid"`
		Score      float64 `json:"score"`
		File       string  `json:"file"`
		Line       *int    `json:"line,omitempty"`
		Title      string  `json:"title"`
		Context    string  `json:"context,omitempty"`
		Body       string  `json:"body,omitempty"`
		Snippet    string  `json:"snippet,omitempty"`
	}

	var output []jsonRow
	for _, row := range results {
		var snippetInfo *store.SnippetResult
		if row.Body != "" {
			info := store.ExtractSnippet(row.Body, opts.Query, 300)
			snippetInfo = &info
		}

		var body, snippet string
		if opts.Full {
			body = row.Body
			if opts.LineNumbers {
				body = store.AddLineNumbers(body, 1)
			}
		} else if snippetInfo != nil {
			snippet = snippetInfo.Snippet
			if opts.LineNumbers {
				snippet = store.AddLineNumbers(snippet, 1)
			}
		}

		var lineVal *int
		if snippetInfo != nil {
			lineVal = &snippetInfo.Line
		}

		output = append(output, jsonRow{
			Docid:   "#" + row.Docid,
			Score:   math.Round(row.Score*100) / 100,
			File:    row.DisplayPath,
			Line:    lineVal,
			Title:   row.Title,
			Context: row.Context,
			Body:    body,
			Snippet: snippet,
		})
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	return string(data)
}

func SearchResultsToCsv(results []store.SearchResult, opts FormatOptions) string {
	var sb strings.Builder
	sb.WriteString("docid,score,file,title,context,line,snippet\n")

	for _, row := range results {
		snippetInfo := store.ExtractSnippet(row.Body, opts.Query, 500)
		content := snippetInfo.Snippet
		if opts.Full {
			content = row.Body
		}
		if opts.LineNumbers && content != "" {
			content = store.AddLineNumbers(content, 1)
		}

		sb.WriteString(fmt.Sprintf(
			"%s,%s,%s,%s,%s,%d,%s\n",
			"#"+row.Docid,
			fmt.Sprintf("%.4f", row.Score),
			EscapeCSV(row.DisplayPath),
			EscapeCSV(row.Title),
			EscapeCSV(row.Context),
			snippetInfo.Line,
			EscapeCSV(content),
		))
	}
	return sb.String()
}

func SearchResultsToFiles(results []store.SearchResult) string {
	var lines []string
	for _, row := range results {
		ctx := ""
		if row.Context != "" {
			ctx = "," + EscapeCSV(row.Context)
		}
		lines = append(lines, fmt.Sprintf("#%s,%.2f,%s%s", row.Docid, row.Score, row.DisplayPath, ctx))
	}
	return strings.Join(lines, "\n")
}

func SearchResultsToMarkdown(results []store.SearchResult, opts FormatOptions) string {
	var blocks []string
	for _, row := range results {
		heading := row.Title
		if heading == "" {
			heading = row.DisplayPath
		}

		var content string
		if opts.Full {
			content = row.Body
		} else {
			content = store.ExtractSnippet(row.Body, opts.Query, 500).Snippet
		}

		if opts.LineNumbers {
			content = store.AddLineNumbers(content, 1)
		}

		fileLine := fmt.Sprintf("**file:** `%s`\n", row.DisplayPath)
		contextLine := ""
		if row.Context != "" {
			contextLine = fmt.Sprintf("**context:** %s\n", row.Context)
		}

		blocks = append(blocks, fmt.Sprintf(
			"---\n# %s\n\n%s**docid:** `#%s`\n%s\n%s\n",
			heading,
			fileLine,
			row.Docid,
			contextLine,
			content,
		))
	}
	return strings.Join(blocks, "\n")
}

func SearchResultsToXml(results []store.SearchResult, opts FormatOptions) string {
	var items []string
	for _, row := range results {
		titleAttr := ""
		if row.Title != "" {
			titleAttr = fmt.Sprintf(" title=%q", EscapeXML(row.Title))
		}
		contextAttr := ""
		if row.Context != "" {
			contextAttr = fmt.Sprintf(" context=%q", EscapeXML(row.Context))
		}

		var content string
		if opts.Full {
			content = row.Body
		} else {
			content = store.ExtractSnippet(row.Body, opts.Query, 500).Snippet
		}
		if opts.LineNumbers {
			content = store.AddLineNumbers(content, 1)
		}

		items = append(items, fmt.Sprintf(
			`<file docid="#%s" name=%q%s%s>
%s
</file>`,
			row.Docid,
			EscapeXML(row.DisplayPath),
			titleAttr,
			contextAttr,
			EscapeXML(content),
		))
	}
	return strings.Join(items, "\n\n")
}

func FormatSearchResults(results []store.SearchResult, format string, opts FormatOptions) string {
	switch format {
	case "json":
		return SearchResultsToJson(results, opts)
	case "csv":
		return SearchResultsToCsv(results, opts)
	case "files":
		return SearchResultsToFiles(results)
	case "md":
		return SearchResultsToMarkdown(results, opts)
	case "xml":
		return SearchResultsToXml(results, opts)
	case "cli":
		return SearchResultsToMarkdown(results, opts)
	default:
		return SearchResultsToJson(results, opts)
	}
}

func DocumentsToJson(results []MultiGetFile) string {
	type docJson struct {
		File    string `json:"file"`
		Title   string `json:"title"`
		Context string `json:"context,omitempty"`
		Skipped bool   `json:"skipped,omitempty"`
		Reason  string `json:"reason,omitempty"`
		Body    string `json:"body,omitempty"`
	}

	var output []docJson
	for _, r := range results {
		row := docJson{
			File:    r.DisplayPath,
			Title:   r.Title,
			Context: r.Context,
		}
		if r.Skipped {
			row.Skipped = true
			row.Reason = r.SkipReason
		} else {
			row.Body = r.Body
		}
		output = append(output, row)
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	return string(data)
}

func DocumentsToCsv(results []MultiGetFile) string {
	var sb strings.Builder
	sb.WriteString("file,title,context,skipped,body\n")
	for _, r := range results {
		skippedStr := "false"
		bodyVal := r.Body
		if r.Skipped {
			skippedStr = "true"
			bodyVal = r.SkipReason
		}
		sb.WriteString(fmt.Sprintf(
			"%s,%s,%s,%s,%s\n",
			EscapeCSV(r.DisplayPath),
			EscapeCSV(r.Title),
			EscapeCSV(r.Context),
			skippedStr,
			EscapeCSV(bodyVal),
		))
	}
	return sb.String()
}

func DocumentsToFiles(results []MultiGetFile) string {
	var lines []string
	for _, r := range results {
		ctx := ""
		if r.Context != "" {
			ctx = "," + EscapeCSV(r.Context)
		}
		status := ""
		if r.Skipped {
			status = ",[SKIPPED]"
		}
		lines = append(lines, fmt.Sprintf("%s%s%s", r.DisplayPath, ctx, status))
	}
	return strings.Join(lines, "\n")
}

func DocumentsToMarkdown(results []MultiGetFile) string {
	var blocks []string
	for _, r := range results {
		md := fmt.Sprintf("## %s\n\n", r.DisplayPath)
		if r.Title != "" && r.Title != r.DisplayPath {
			md += fmt.Sprintf("**Title:** %s\n\n", r.Title)
		}
		if r.Context != "" {
			md += fmt.Sprintf("**Context:** %s\n\n", r.Context)
		}
		if r.Skipped {
			md += fmt.Sprintf("> %s\n", r.SkipReason)
		} else {
			md += "```\n" + r.Body + "\n```\n"
		}
		blocks = append(blocks, md)
	}
	return strings.Join(blocks, "\n")
}

func DocumentsToXml(results []MultiGetFile) string {
	var items []string
	for _, r := range results {
		var content string
		if r.Skipped {
			content = fmt.Sprintf("    <skipped>true</skipped>\n    <reason>%s</reason>", xmlEscapeString(r.SkipReason))
		} else {
			content = fmt.Sprintf("    <body>%s</body>", xmlEscapeString(r.Body))
		}

		ctxTag := ""
		if r.Context != "" {
			ctxTag = fmt.Sprintf("\n    <context>%s</context>", xmlEscapeString(r.Context))
		}

		items = append(items, fmt.Sprintf(
			`  <document>
    <file>%s</file>
    <title>%s</title>%s
%s
  </document>`,
			xmlEscapeString(r.DisplayPath),
			xmlEscapeString(r.Title),
			ctxTag,
			content,
		))
	}
	return fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<documents>\n%s\n</documents>", strings.Join(items, "\n"))
}

func xmlEscapeString(s string) string {
	var buf strings.Builder
	xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func FormatDocuments(results []MultiGetFile, format string) string {
	switch format {
	case "json":
		return DocumentsToJson(results)
	case "csv":
		return DocumentsToCsv(results)
	case "files":
		return DocumentsToFiles(results)
	case "md":
		return DocumentsToMarkdown(results)
	case "xml":
		return DocumentsToXml(results)
	case "cli":
		return DocumentsToMarkdown(results)
	default:
		return DocumentsToJson(results)
	}
}

type DocumentResult struct {
	DisplayPath string `json:"displayPath"`
	Title       string `json:"title"`
	Context     string `json:"context,omitempty"`
	Hash        string `json:"hash"`
	ModifiedAt  string `json:"modifiedAt"`
	BodyLength  int    `json:"bodyLength"`
	Body        string `json:"body,omitempty"`
}

func DocumentToJson(doc DocumentResult) string {
	data, _ := json.MarshalIndent(doc, "", "  ")
	return string(data)
}

func DocumentToMarkdown(doc DocumentResult) string {
	md := fmt.Sprintf("# %s\n\n", doc.Title)
	if doc.Context != "" {
		md += fmt.Sprintf("**Context:** %s\n\n", doc.Context)
	}
	md += fmt.Sprintf("**File:** %s\n", doc.DisplayPath)
	md += fmt.Sprintf("**Modified:** %s\n\n", doc.ModifiedAt)
	if doc.Body != "" {
		md += "---\n\n" + doc.Body + "\n"
	}
	return md
}

func DocumentToXml(doc DocumentResult) string {
	xmlStr := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<document>\n"
	xmlStr += fmt.Sprintf("  <file>%s</file>\n", xmlEscapeString(doc.DisplayPath))
	xmlStr += fmt.Sprintf("  <title>%s</title>\n", xmlEscapeString(doc.Title))
	if doc.Context != "" {
		xmlStr += fmt.Sprintf("  <context>%s</context>\n", xmlEscapeString(doc.Context))
	}
	xmlStr += fmt.Sprintf("  <hash>%s</hash>\n", xmlEscapeString(doc.Hash))
	xmlStr += fmt.Sprintf("  <modifiedAt>%s</modifiedAt>\n", xmlEscapeString(doc.ModifiedAt))
	xmlStr += fmt.Sprintf("  <bodyLength>%d</bodyLength>\n", doc.BodyLength)
	if doc.Body != "" {
		xmlStr += fmt.Sprintf("  <body>%s</body>\n", xmlEscapeString(doc.Body))
	}
	xmlStr += "</document>"
	return xmlStr
}

func FormatDocument(doc DocumentResult, format string) string {
	switch format {
	case "json":
		return DocumentToJson(doc)
	case "md":
		return DocumentToMarkdown(doc)
	case "xml":
		return DocumentToXml(doc)
	default:
		return DocumentToMarkdown(doc)
	}
}
