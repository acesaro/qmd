package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type SupportedLanguage string

const (
	LangTypeScript SupportedLanguage = "typescript"
	LangTSX        SupportedLanguage = "tsx"
	LangJavaScript SupportedLanguage = "javascript"
	LangPython     SupportedLanguage = "python"
	LangGo         SupportedLanguage = "go"
	LangRust       SupportedLanguage = "rust"
	LangSQL        SupportedLanguage = "sql"
	LangKQL        SupportedLanguage = "kql"
	LangNone       SupportedLanguage = ""
)

func detectLanguage(filePath string) SupportedLanguage {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".ts", ".mts", ".cts":
		return LangTypeScript
	case ".tsx":
		return LangTSX
	case ".js", ".mjs", ".cjs":
		return LangJavaScript
	case ".jsx":
		return LangTSX
	case ".py":
		return LangPython
	case ".go":
		return LangGo
	case ".rs":
		return LangRust
	case ".sql":
		return LangSQL
	case ".kql", ".csl":
		return LangKQL
	default:
		return LangNone
	}
}

// Regex definitions for languages
var (
	// JS/TS
	tsImportRegex = regexp.MustCompile(`^import\b`)
	tsExportRegex = regexp.MustCompile(`^export\b`)
	tsClassRegex  = regexp.MustCompile(`^(export\s+)?(class|interface)\b`)
	tsTypeRegex   = regexp.MustCompile(`^(export\s+)?(type|enum)\b`)
	tsFuncRegex   = regexp.MustCompile(`^(export\s+)?(async\s+)?function\b`)
	tsArrowRegex  = regexp.MustCompile(`^(export\s+)?(const|let|var)\s+(\w+)\s*=\s*(async\s*)?\([^)]*\)\s*=>`)
	tsFnExprRegex = regexp.MustCompile(`^(export\s+)?(const|let|var)\s+(\w+)\s*=\s*(async\s*)?function\b`)
	tsMethodRegex = regexp.MustCompile(`^\s*(async\s+|private\s+|public\s+|protected\s+|static\s+)*\w+\s*\([^)]*\)\s*(:\s*[^{]+)?\s*\{`)

	// Python
	pyImportRegex    = regexp.MustCompile(`^(import\s|from\s)`)
	pyClassRegex     = regexp.MustCompile(`^\s*class\s+(\w+)`)
	pyFuncRegex      = regexp.MustCompile(`^\s*(async\s+)?def\s+(\w+)`)
	pyDecoratorRegex = regexp.MustCompile(`^\s*@[\w.]+`)

	// Rust
	rsUseRegex    = regexp.MustCompile(`^use\s`)
	rsStructRegex = regexp.MustCompile(`^(pub\s+)?(struct|impl|trait|enum|mod)\b`)
	rsFnRegex     = regexp.MustCompile(`^(pub(\([^)]*\))?\s*)?(const\s+)?(async\s+)?fn\b`)

	// SQL
	sqlStructRegex = regexp.MustCompile(`(?i)^\s*create\s+(table|view|schema|database|table\s+if\s+not\s+exists)\b`)
	sqlFuncRegex   = regexp.MustCompile(`(?i)^\s*create\s+(or\s+replace\s+)?(function|procedure|trigger)\b`)
	sqlTypeRegex   = regexp.MustCompile(`(?i)^\s*create\s+type\b`)

	// KQL
	kqlFuncRegex = regexp.MustCompile(`(?i)^\s*let\s+\w+\s*=\s*(\([^)]*\)\s*\{)?`)
)

func GetASTBreakPoints(content string, filePath string) []BreakPoint {
	lang := detectLanguage(filePath)
	if lang == LangNone || content == "" {
		return nil
	}

	switch lang {
	case LangGo:
		return getGoBreakPoints(content)
	case LangTypeScript, LangTSX, LangJavaScript:
		return getTsJsBreakPoints(content)
	case LangPython:
		return getPythonBreakPoints(content)
	case LangRust:
		return getRustBreakPoints(content)
	case LangSQL:
		return getSqlBreakPoints(content)
	case LangKQL:
		return getKqlBreakPoints(content)
	}

	return nil
}

func getGoBreakPoints(content string) []BreakPoint {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ImportsOnly|parser.ParseComments)
	_ = file // Parser was run partially, let's parse fully
	file, err = parser.ParseFile(fset, "", content, parser.AllErrors)
	if err != nil {
		// Fallback to minimal regex scan if syntax is completely broken
		return getGoRegexBreakPoints(content)
	}

	var points []BreakPoint
	seen := make(map[int]bool)

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		pos := fset.Position(n.Pos()).Offset
		if seen[pos] {
			return true
		}

		switch node := n.(type) {
		case *ast.FuncDecl:
			seen[pos] = true
			if node.Recv == nil {
				points = append(points, BreakPoint{Pos: pos, Score: 90, Type: "ast:func"})
			} else {
				points = append(points, BreakPoint{Pos: pos, Score: 90, Type: "ast:method"})
			}
		case *ast.GenDecl:
			if node.Tok == token.TYPE {
				seen[pos] = true
				points = append(points, BreakPoint{Pos: pos, Score: 80, Type: "ast:type"})
			} else if node.Tok == token.IMPORT {
				seen[pos] = true
				points = append(points, BreakPoint{Pos: pos, Score: 60, Type: "ast:import"})
			}
		}
		return true
	})

	sort.Slice(points, func(i, j int) bool {
		return points[i].Pos < points[j].Pos
	})
	return points
}

func getGoRegexBreakPoints(content string) []BreakPoint {
	var points []BreakPoint
	lines := strings.Split(content, "\n")
	offset := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") {
			if strings.Contains(trimmed, ")") && strings.HasPrefix(strings.TrimPrefix(trimmed, "func "), "(") {
				points = append(points, BreakPoint{Pos: offset, Score: 90, Type: "ast:method"})
			} else {
				points = append(points, BreakPoint{Pos: offset, Score: 90, Type: "ast:func"})
			}
		} else if strings.HasPrefix(trimmed, "type ") {
			points = append(points, BreakPoint{Pos: offset, Score: 80, Type: "ast:type"})
		} else if strings.HasPrefix(trimmed, "import ") {
			points = append(points, BreakPoint{Pos: offset, Score: 60, Type: "ast:import"})
		}
		offset += len(line) + 1
	}
	return points
}

func getTsJsBreakPoints(content string) []BreakPoint {
	var points []BreakPoint
	lines := strings.Split(content, "\n")
	offset := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			offset += len(line) + 1
			continue
		}

		if tsClassRegex.MatchString(trimmed) {
			t := "ast:class"
			if strings.Contains(trimmed, "interface") {
				t = "ast:iface"
			}
			points = append(points, BreakPoint{Pos: offset, Score: 100, Type: t})
		} else if tsFuncRegex.MatchString(trimmed) || tsArrowRegex.MatchString(trimmed) || tsFnExprRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 90, Type: "ast:func"})
		} else if tsMethodRegex.MatchString(line) { // match original line to handle indentation
			// Methods score 90
			points = append(points, BreakPoint{Pos: offset, Score: 90, Type: "ast:method"})
		} else if tsTypeRegex.MatchString(trimmed) {
			t := "ast:type"
			if strings.Contains(trimmed, "enum") {
				t = "ast:enum"
			}
			points = append(points, BreakPoint{Pos: offset, Score: 80, Type: t})
		} else if tsImportRegex.MatchString(trimmed) || tsExportRegex.MatchString(trimmed) {
			t := "ast:import"
			score := 60
			if strings.HasPrefix(trimmed, "export") {
				t = "ast:export"
				score = 90
			}
			points = append(points, BreakPoint{Pos: offset, Score: score, Type: t})
		}

		offset += len(line) + 1
	}

	return points
}

func getPythonBreakPoints(content string) []BreakPoint {
	var points []BreakPoint
	lines := strings.Split(content, "\n")
	offset := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			offset += len(line) + 1
			continue
		}

		if pyClassRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 100, Type: "ast:class"})
		} else if pyFuncRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 90, Type: "ast:func"})
		} else if pyDecoratorRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 90, Type: "ast:decorated"})
		} else if pyImportRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 60, Type: "ast:import"})
		}

		offset += len(line) + 1
	}

	return points
}

func getRustBreakPoints(content string) []BreakPoint {
	var points []BreakPoint
	lines := strings.Split(content, "\n")
	offset := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			offset += len(line) + 1
			continue
		}

		if rsStructRegex.MatchString(trimmed) {
			t := "ast:struct"
			if strings.Contains(trimmed, "impl") {
				t = "ast:impl"
			} else if strings.Contains(trimmed, "trait") {
				t = "ast:trait"
			} else if strings.Contains(trimmed, "enum") {
				t = "ast:enum"
			} else if strings.Contains(trimmed, "mod") {
				t = "ast:mod"
			}
			points = append(points, BreakPoint{Pos: offset, Score: 100, Type: t})
		} else if rsFnRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 90, Type: "ast:func"})
		} else if rsUseRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 60, Type: "ast:import"})
		}

		offset += len(line) + 1
	}

	return points
}

func mergeBreakPoints(a, b []BreakPoint) []BreakPoint {
	seen := make(map[int]BreakPoint)
	for _, bp := range a {
		existing, ok := seen[bp.Pos]
		if !ok || bp.Score > existing.Score {
			seen[bp.Pos] = bp
		}
	}
	for _, bp := range b {
		existing, ok := seen[bp.Pos]
		if !ok || bp.Score > existing.Score {
			seen[bp.Pos] = bp
		}
	}

	var merged []BreakPoint
	for _, bp := range seen {
		merged = append(merged, bp)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Pos < merged[j].Pos
	})
	return merged
}

func getSqlBreakPoints(content string) []BreakPoint {
	var points []BreakPoint
	lines := strings.Split(content, "\n")
	offset := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			offset += len(line) + 1
			continue
		}

		if sqlStructRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 100, Type: "ast:struct"})
		} else if sqlFuncRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 90, Type: "ast:func"})
		} else if sqlTypeRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 80, Type: "ast:type"})
		}

		offset += len(line) + 1
	}
	return points
}

func getKqlBreakPoints(content string) []BreakPoint {
	var points []BreakPoint
	lines := strings.Split(content, "\n")
	offset := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			offset += len(line) + 1
			continue
		}

		if kqlFuncRegex.MatchString(trimmed) {
			points = append(points, BreakPoint{Pos: offset, Score: 90, Type: "ast:func"})
		}

		offset += len(line) + 1
	}
	return points
}
