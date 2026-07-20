package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

func emojiToHex(s string) string {
	var sb strings.Builder
	runes := []rune(s)
	n := len(runes)
	i := 0
	for i < n {
		if unicode.Is(unicode.So, runes[i]) || unicode.Is(unicode.Sk, runes[i]) {
			var run []rune
			for i < n && (unicode.Is(unicode.So, runes[i]) || unicode.Is(unicode.Sk, runes[i]) || unicode.Is(unicode.Mn, runes[i])) {
				if unicode.Is(unicode.So, runes[i]) || unicode.Is(unicode.Sk, runes[i]) {
					run = append(run, runes[i])
				}
				i++
			}
			var hexParts []string
			for _, r := range run {
				hexParts = append(hexParts, fmt.Sprintf("%x", r))
			}
			sb.WriteString(strings.Join(hexParts, "-"))
		} else {
			sb.WriteRune(runes[i])
			i++
		}
	}
	return sb.String()
}

func cleanSegment(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '$' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	res := sb.String()

	var finalSb strings.Builder
	lastDash := false
	for _, r := range res {
		if r == '-' {
			if !lastDash {
				finalSb.WriteRune(r)
				lastDash = true
			}
		} else {
			finalSb.WriteRune(r)
			lastDash = false
		}
	}
	res = finalSb.String()
	return strings.Trim(res, "-")
}

func Handelize(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("handelize: path cannot be empty")
	}

	segments := strings.Split(path, "/")
	var nonESegments []string
	for _, s := range segments {
		if s != "" {
			nonESegments = append(nonESegments, s)
		}
	}
	if len(nonESegments) == 0 {
		return "", errors.New("handelize: path has no valid filename content")
	}

	lastSegment := nonESegments[len(nonESegments)-1]
	ext := filepath.Ext(lastSegment)
	filenameWithoutExt := strings.TrimSuffix(lastSegment, ext)

	hasValidContent := false
	for _, r := range filenameWithoutExt {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r) || r == '$' {
			hasValidContent = true
			break
		}
	}
	if !hasValidContent {
		return "", fmt.Errorf("handelize: path %q has no valid filename content", path)
	}

	normalizedPath := strings.ReplaceAll(path, "___", "/")
	segments = strings.Split(normalizedPath, "/")
	var cleanedSegments []string

	for idx, segment := range segments {
		if segment == "" {
			continue
		}
		isLastSegment := idx == len(segments)-1
		segment = emojiToHex(segment)

		var cleaned string
		if isLastSegment {
			extMatch := filepath.Ext(segment)
			nameWithoutExt := strings.TrimSuffix(segment, extMatch)
			cleanedName := cleanSegment(nameWithoutExt)
			cleaned = cleanedName + extMatch
		} else {
			cleaned = cleanSegment(segment)
		}

		if cleaned != "" {
			cleanedSegments = append(cleanedSegments, cleaned)
		}
	}

	result := strings.Join(cleanedSegments, "/")
	if result == "" {
		return "", fmt.Errorf("handelize: path %q resulted in empty string after processing", path)
	}

	return result, nil
}
