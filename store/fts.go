package store

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	rxCjkChar   = regexp.MustCompile(`[\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}]`)
	rxCjkRun    = regexp.MustCompile(`[\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}]+`)
	rxSanitize  = regexp.MustCompile(`[^\p{L}\p{N}'_]`)
	rxHyphenated = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}'-]*-[\p{L}\p{N}][\p{L}\p{N}'-]*$`)
	rxWordOrNum  = regexp.MustCompile(`^[\p{L}\p{N}_]+$`)
)

func NormalizeCjkForFTS(text string) string {
	return rxCjkRun.ReplaceAllStringFunc(text, func(run string) string {
		runRunes := []rune(run)
		var sb strings.Builder
		sb.WriteString(" ")
		for i, r := range runRunes {
			if i > 0 {
				sb.WriteString(" ")
			}
			sb.WriteRune(r)
		}
		sb.WriteString(" ")
		return sb.String()
	})
}

func containsCjk(text string) bool {
	return rxCjkChar.MatchString(text)
}

func SanitizeFTS5Term(term string) string {
	return strings.ToLower(rxSanitize.ReplaceAllString(term, ""))
}

func sanitizeFTS5Phrase(phrase string) string {
	normalized := NormalizeCjkForFTS(phrase)
	parts := strings.Fields(normalized)
	var sanitized []string
	for _, part := range parts {
		s := SanitizeFTS5Term(part)
		if s != "" {
			sanitized = append(sanitized, s)
		}
	}
	return strings.Join(sanitized, " ")
}

func isHyphenatedToken(token string) bool {
	return rxHyphenated.MatchString(token)
}

func sanitizeHyphenatedTerm(term string) string {
	parts := strings.Split(term, "-")
	var sanitized []string
	for _, part := range parts {
		s := SanitizeFTS5Term(part)
		if s != "" {
			sanitized = append(sanitized, s)
		}
	}
	return strings.Join(sanitized, " ")
}

func isDottedToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || !rxWordOrNum.MatchString(part) {
			return false
		}
	}
	return true
}

func sanitizeDottedTerm(term string) string {
	parts := strings.Split(term, ".")
	var sanitized []string
	for _, part := range parts {
		s := SanitizeFTS5Term(part)
		if s != "" {
			sanitized = append(sanitized, `"`+s+`"*`)
		}
	}
	return strings.Join(sanitized, " AND ")
}

func BuildFTS5Query(query string) string {
	var positive []string
	var negative []string

	runes := []rune(query)
	n := len(runes)
	i := 0

	for i < n {
		// Skip whitespace
		for i < n && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= n {
			break
		}

		// Check for negation prefix
		negated := runes[i] == '-'
		if negated {
			i++
		}

		if i >= n {
			break
		}

		// Check for quoted phrase
		if runes[i] == '"' {
			start := i + 1
			i++
			for i < n && runes[i] != '"' {
				i++
			}
			phrase := string(runes[start:i])
			if i < n {
				i++ // skip closing quote
			}
			phrase = strings.TrimSpace(phrase)
			if len(phrase) > 0 {
				sanitized := sanitizeFTS5Phrase(phrase)
				if sanitized != "" {
					ftsPhrase := `"` + sanitized + `"`
					if negated {
						negative = append(negative, ftsPhrase)
					} else {
						positive = append(positive, ftsPhrase)
					}
				}
			}
		} else {
			// Plain term
			start := i
			for i < n && !unicode.IsSpace(runes[i]) && runes[i] != '"' {
				i++
			}
			term := string(runes[start:i])

			if isHyphenatedToken(term) {
				sanitized := sanitizeHyphenatedTerm(term)
				if sanitized != "" {
					ftsPhrase := `"` + sanitized + `"`
					if negated {
						negative = append(negative, ftsPhrase)
					} else {
						positive = append(positive, ftsPhrase)
					}
				}
			} else if isDottedToken(term) {
				sanitized := sanitizeDottedTerm(term)
				if sanitized != "" {
					if negated {
						negative = append(negative, "("+sanitized+")")
					} else {
						for _, part := range strings.Split(sanitized, " AND ") {
							positive = append(positive, strings.TrimSpace(part))
						}
					}
				}
			} else if containsCjk(term) {
				sanitized := sanitizeFTS5Phrase(term)
				if sanitized != "" {
					ftsPhrase := `"` + sanitized + `"`
					if negated {
						negative = append(negative, ftsPhrase)
					} else {
						positive = append(positive, ftsPhrase)
					}
				}
			} else {
				sanitized := SanitizeFTS5Term(term)
				if sanitized != "" {
					ftsTerm := `"` + sanitized + `"*`
					if negated {
						negative = append(negative, ftsTerm)
					} else {
						positive = append(positive, ftsTerm)
					}
				}
			}
		}
	}

	if len(positive) == 0 && len(negative) == 0 {
		return ""
	}

	if len(positive) == 0 {
		return "" // FTS5 NOT is a binary operator, needs positive terms
	}

	result := strings.Join(positive, " AND ")
	for _, neg := range negative {
		result = result + " NOT " + neg
	}

	return result
}
