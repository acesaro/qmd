package store

import (
	"regexp"
	"strings"
)

// GlobToRegex converts a glob pattern (with ** support) to a Go regular expression.
func GlobToRegex(pattern string) (*regexp.Regexp, error) {
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	var sb strings.Builder
	sb.WriteString("^")

	i := 0
	n := len(pattern)
	for i < n {
		if i+3 <= n && pattern[i:i+3] == "**/" {
			sb.WriteString("(?:.*/)?")
			i += 3
		} else if i+2 <= n && pattern[i:i+2] == "**" {
			sb.WriteString(".*")
			i += 2
		} else if pattern[i] == '*' {
			sb.WriteString("[^/]*")
			i++
		} else if pattern[i] == '?' {
			sb.WriteString("[^/]")
			i++
		} else if strings.ContainsRune(".+()|{}+^$[]\\", rune(pattern[i])) {
			sb.WriteByte('\\')
			sb.WriteByte(pattern[i])
			i++
		} else {
			sb.WriteByte(pattern[i])
			i++
		}
	}
	sb.WriteString("$")
	return regexp.Compile(sb.String())
}

// MatchPath checks if a path matches a glob pattern.
func MatchPath(pattern, path string) bool {
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	path = strings.ReplaceAll(path, "\\", "/")

	re, err := GlobToRegex(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(path)
}
