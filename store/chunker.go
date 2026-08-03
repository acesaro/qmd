package store

import (
	"regexp"
	"sort"
)

const (
	ChunkSizeTokens    = 900
	ChunkOverlapTokens = 135
	ChunkSizeChars     = ChunkSizeTokens * 4     // 3600
	ChunkOverlapChars  = ChunkOverlapTokens * 4  // 540
	ChunkWindowChars   = 200 * 4                 // 800
)

type BreakPoint struct {
	Pos   int
	Score int
	Type  string
}

type CodeFenceRegion struct {
	Start int
	End   int
}

type Chunk struct {
	Text string
	Pos  int
}

// BREAK_PATTERNS in Go. Since we don't have lookaheads in Go's standard regexp,
// we will compile equivalent regexes or parse them carefully.
var (
	rxHeadings    = regexp.MustCompile(`\n(#+)([^#\n]|$)`)
	rxCodeBlock   = regexp.MustCompile(`\n` + "```")
	rxHr          = regexp.MustCompile(`\n(?:---|\*\*\*|___)\s*\n`)
	rxBlank       = regexp.MustCompile(`\n\n+`)
	rxList        = regexp.MustCompile(`\n[-*]\s`)
	rxNumList     = regexp.MustCompile(`\n\d+\.\s`)
	rxNewline     = regexp.MustCompile(`\n`)
	rxCodeFence   = regexp.MustCompile(`\n` + "```")
)

func ScanBreakPoints(text string) []BreakPoint {
	seen := make(map[int]BreakPoint)

	// 1. Headings
	for _, m := range rxHeadings.FindAllStringSubmatchIndex(text, -1) {
		// m[0] is start of match, m[1] is end of match.
		// m[2], m[3] is indexes for group 1 (#+)
		pos := m[0]
		hashesLen := m[3] - m[2]
		score := 50
		typeName := "h6"
		switch hashesLen {
		case 1:
			score = 100
			typeName = "h1"
		case 2:
			score = 90
			typeName = "h2"
		case 3:
			score = 80
			typeName = "h3"
		case 4:
			score = 70
			typeName = "h4"
		case 5:
			score = 60
			typeName = "h5"
		}
		seen[pos] = BreakPoint{Pos: pos, Score: score, Type: typeName}
	}

	// Helper to add matches
	addMatches := func(rx *regexp.Regexp, score int, typeName string) {
		for _, m := range rx.FindAllStringIndex(text, -1) {
			pos := m[0]
			if existing, ok := seen[pos]; !ok || score > existing.Score {
				seen[pos] = BreakPoint{Pos: pos, Score: score, Type: typeName}
			}
		}
	}

	addMatches(rxCodeBlock, 80, "codeblock")
	addMatches(rxHr, 60, "hr")
	addMatches(rxBlank, 20, "blank")
	addMatches(rxList, 5, "list")
	addMatches(rxNumList, 5, "numlist")
	addMatches(rxNewline, 1, "newline")

	// Convert to slice and sort
	var list []BreakPoint
	for _, bp := range seen {
		list = append(list, bp)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Pos < list[j].Pos
	})

	return list
}

func FindCodeFences(text string) []CodeFenceRegion {
	var regions []CodeFenceRegion
	inFence := false
	fenceStart := 0

	for _, m := range rxCodeFence.FindAllStringIndex(text, -1) {
		if !inFence {
			fenceStart = m[0]
			inFence = true
		} else {
			regions = append(regions, CodeFenceRegion{Start: fenceStart, End: m[1]})
			inFence = false
		}
	}

	if inFence {
		regions = append(regions, CodeFenceRegion{Start: fenceStart, End: len(text)})
	}

	return regions
}

func IsInsideCodeFence(pos int, fences []CodeFenceRegion) bool {
	for _, f := range fences {
		if pos > f.Start && pos < f.End {
			return true
		}
	}
	return false
}

func FindBestCutoff(
	breakPoints []BreakPoint,
	targetCharPos int,
	windowChars int,
	decayFactor float64,
	codeFences []CodeFenceRegion,
) int {
	windowStart := targetCharPos - windowChars
	bestScore := -1.0
	bestPos := targetCharPos

	for _, bp := range breakPoints {
		if bp.Pos < windowStart {
			continue
		}
		if bp.Pos > targetCharPos {
			break // sorted, safe to stop
		}

		if IsInsideCodeFence(bp.Pos, codeFences) {
			continue
		}

		distance := targetCharPos - bp.Pos
		normalizedDist := float64(distance) / float64(windowChars)
		multiplier := 1.0 - (normalizedDist*normalizedDist)*decayFactor
		finalScore := float64(bp.Score) * multiplier

		if finalScore > bestScore {
			bestScore = finalScore
			bestPos = bp.Pos
		}
	}

	return bestPos
}

func ChunkDocumentWithBreakPoints(
	content string,
	breakPoints []BreakPoint,
	codeFences []CodeFenceRegion,
	maxChars int,
	overlapChars int,
	windowChars int,
) []Chunk {
	if len(content) <= maxChars {
		return []Chunk{{Text: content, Pos: 0}}
	}

	var chunks []Chunk
	charPos := 0

	for charPos < len(content) {
		targetEndPos := charPos + maxChars
		if targetEndPos > len(content) {
			targetEndPos = len(content)
		}
		endPos := targetEndPos

		if endPos < len(content) {
			bestCutoff := FindBestCutoff(
				breakPoints,
				targetEndPos,
				windowChars,
				0.7,
				codeFences,
			)

			if bestCutoff > charPos && bestCutoff <= targetEndPos {
				endPos = bestCutoff
			}
		}

		if endPos <= charPos {
			endPos = charPos + maxChars
			if endPos > len(content) {
				endPos = len(content)
			}
		}

		chunks = append(chunks, Chunk{
			Text: content[charPos:endPos],
			Pos:  charPos,
		})

		if endPos >= len(content) {
			break
		}
		charPos = endPos - overlapChars
		lastChunkPos := chunks[len(chunks)-1].Pos
		if charPos <= lastChunkPos {
			charPos = endPos
		}
	}

	return chunks
}

func ChunkDocument(content string) []Chunk {
	return ChunkDocumentWithStrategy(content, "", "regex")
}

func ChunkDocumentWithStrategy(content string, filePath string, strategy string) []Chunk {
	bps := ScanBreakPoints(content)
	fences := FindCodeFences(content)
	if strategy == "auto" && filePath != "" {
		astBps := GetASTBreakPoints(content, filePath)
		if len(astBps) > 0 {
			bps = mergeBreakPoints(bps, astBps)
		}
	}
	return ChunkDocumentWithBreakPoints(
		content,
		bps,
		fences,
		ChunkSizeChars,
		ChunkOverlapChars,
		ChunkWindowChars,
	)
}
