package store

import (
	"strings"
	"testing"
)

func TestScanBreakPoints(t *testing.T) {
	t.Run("detects h1 headings", func(t *testing.T) {
		text := "Intro\n# Heading 1\nMore text"
		breaks := ScanBreakPoints(text)
		var h1 *BreakPoint
		for _, b := range breaks {
			if b.Type == "h1" {
				h1 = &b
				break
			}
		}
		if h1 == nil {
			t.Fatal("expected h1 heading to be detected")
		}
		if h1.Score != 100 {
			t.Errorf("expected score 100, got %d", h1.Score)
		}
		if h1.Pos != 5 {
			t.Errorf("expected position 5, got %d", h1.Pos)
		}
	})

	t.Run("detects multiple heading levels", func(t *testing.T) {
		text := "Text\n# H1\n## H2\n### H3\nMore"
		breaks := ScanBreakPoints(text)

		var h1, h2, h3 *BreakPoint
		for i, b := range breaks {
			switch b.Type {
			case "h1":
				h1 = &breaks[i]
			case "h2":
				h2 = &breaks[i]
			case "h3":
				h3 = &breaks[i]
			}
		}

		if h1 == nil || h2 == nil || h3 == nil {
			t.Fatal("expected h1, h2, and h3 to be detected")
		}
		if h1.Score != 100 || h2.Score != 90 || h3.Score != 80 {
			t.Errorf("unexpected scores: h1=%d, h2=%d, h3=%d", h1.Score, h2.Score, h3.Score)
		}
	})

	t.Run("detects code blocks", func(t *testing.T) {
		text := "Before\n```js\ncode\n```\nAfter"
		breaks := ScanBreakPoints(text)
		var codeblocks []BreakPoint
		for _, b := range breaks {
			if b.Type == "codeblock" {
				codeblocks = append(codeblocks, b)
			}
		}
		if len(codeblocks) != 2 {
			t.Fatalf("expected 2 codeblock breaks, got %d", len(codeblocks))
		}
		if codeblocks[0].Score != 80 {
			t.Errorf("expected score 80, got %d", codeblocks[0].Score)
		}
	})

	t.Run("detects horizontal rules", func(t *testing.T) {
		text := "Text\n---\nMore text"
		breaks := ScanBreakPoints(text)
		var hr *BreakPoint
		for _, b := range breaks {
			if b.Type == "hr" {
				hr = &b
				break
			}
		}
		if hr == nil {
			t.Fatal("expected hr to be detected")
		}
		if hr.Score != 60 {
			t.Errorf("expected score 60, got %d", hr.Score)
		}
	})

	t.Run("detects blank lines (paragraph boundaries)", func(t *testing.T) {
		text := "First paragraph.\n\nSecond paragraph."
		breaks := ScanBreakPoints(text)
		var blank *BreakPoint
		for _, b := range breaks {
			if b.Type == "blank" {
				blank = &b
				break
			}
		}
		if blank == nil {
			t.Fatal("expected blank line to be detected")
		}
		if blank.Score != 20 {
			t.Errorf("expected score 20, got %d", blank.Score)
		}
	})

	t.Run("detects list items", func(t *testing.T) {
		text := "Intro\n- Item 1\n- Item 2\n1. Numbered"
		breaks := ScanBreakPoints(text)

		var lists []BreakPoint
		var numlists []BreakPoint
		for _, b := range breaks {
			if b.Type == "list" {
				lists = append(lists, b)
			} else if b.Type == "numlist" {
				numlists = append(numlists, b)
			}
		}

		if len(lists) != 2 || len(numlists) != 1 {
			t.Fatalf("expected 2 lists and 1 numlist, got %d and %d", len(lists), len(numlists))
		}
		if lists[0].Score != 5 || numlists[0].Score != 5 {
			t.Errorf("unexpected scores: list=%d, numlist=%d", lists[0].Score, numlists[0].Score)
		}
	})

	t.Run("detects newlines as fallback", func(t *testing.T) {
		text := "Line 1\nLine 2\nLine 3"
		breaks := ScanBreakPoints(text)
		var newlines []BreakPoint
		for _, b := range breaks {
			if b.Type == "newline" {
				newlines = append(newlines, b)
			}
		}
		if len(newlines) != 2 {
			t.Fatalf("expected 2 newlines, got %d", len(newlines))
		}
		if newlines[0].Score != 1 {
			t.Errorf("expected score 1, got %d", newlines[0].Score)
		}
	})

	t.Run("returns breaks sorted by position", func(t *testing.T) {
		text := "A\n# B\n\nC\n## D"
		breaks := ScanBreakPoints(text)
		for i := 1; i < len(breaks); i++ {
			if breaks[i].Pos <= breaks[i-1].Pos {
				t.Errorf("breaks are not sorted by position: %d and %d", breaks[i-1].Pos, breaks[i].Pos)
			}
		}
	})

	t.Run("higher-scoring pattern wins at same position", func(t *testing.T) {
		text := "Text\n# Heading"
		breaks := ScanBreakPoints(text)
		var atPos []BreakPoint
		for _, b := range breaks {
			if b.Pos == 4 {
				atPos = append(atPos, b)
			}
		}
		if len(atPos) != 1 {
			t.Fatalf("expected 1 break point at position 4, got %d", len(atPos))
		}
		if atPos[0].Type != "h1" || atPos[0].Score != 100 {
			t.Errorf("unexpected break point: %+v", atPos[0])
		}
	})
}

func TestFindCodeFences(t *testing.T) {
	t.Run("finds single code fence", func(t *testing.T) {
		text := "Before\n```js\ncode here\n```\nAfter"
		fences := FindCodeFences(text)
		if len(fences) != 1 {
			t.Fatalf("expected 1 fence, got %d", len(fences))
		}
		if fences[0].Start != 6 {
			t.Errorf("expected start 6, got %d", fences[0].Start)
		}
		if fences[0].End != 26 {
			t.Errorf("expected end 26, got %d", fences[0].End)
		}
	})

	t.Run("finds multiple code fences", func(t *testing.T) {
		text := "Intro\n```\nblock1\n```\nMiddle\n```\nblock2\n```\nEnd"
		fences := FindCodeFences(text)
		if len(fences) != 2 {
			t.Errorf("expected 2 fences, got %d", len(fences))
		}
	})

	t.Run("handles unclosed code fence", func(t *testing.T) {
		text := "Before\n```\nunclosed code block"
		fences := FindCodeFences(text)
		if len(fences) != 1 {
			t.Fatalf("expected 1 fence, got %d", len(fences))
		}
		if fences[0].End != len(text) {
			t.Errorf("expected end %d, got %d", len(text), fences[0].End)
		}
	})

	t.Run("returns empty array for no code fences", func(t *testing.T) {
		text := "No code fences here"
		fences := FindCodeFences(text)
		if len(fences) != 0 {
			t.Errorf("expected 0 fences, got %d", len(fences))
		}
	})
}

func TestIsInsideCodeFence(t *testing.T) {
	fences := []CodeFenceRegion{{Start: 10, End: 30}}

	if !IsInsideCodeFence(15, fences) {
		t.Error("expected 15 to be inside fence")
	}
	if IsInsideCodeFence(5, fences) {
		t.Error("expected 5 to be outside fence")
	}
	if IsInsideCodeFence(10, fences) {
		t.Error("expected 10 (boundary) to be outside fence")
	}
	if IsInsideCodeFence(30, fences) {
		t.Error("expected 30 (boundary) to be outside fence")
	}
}

func TestFindBestCutoff(t *testing.T) {
	t.Run("prefers higher-scoring break points", func(t *testing.T) {
		breakPoints := []BreakPoint{
			{Pos: 100, Score: 1, Type: "newline"},
			{Pos: 150, Score: 100, Type: "h1"},
			{Pos: 180, Score: 20, Type: "blank"},
		}
		cutoff := FindBestCutoff(breakPoints, 200, 100, 0.7, nil)
		if cutoff != 150 {
			t.Errorf("expected cutoff 150, got %d", cutoff)
		}
	})

	t.Run("h2 at window edge beats blank at target", func(t *testing.T) {
		breakPoints := []BreakPoint{
			{Pos: 100, Score: 90, Type: "h2"},
			{Pos: 195, Score: 20, Type: "blank"},
		}
		cutoff := FindBestCutoff(breakPoints, 200, 100, 0.7, nil)
		if cutoff != 100 {
			t.Errorf("expected cutoff 100, got %d", cutoff)
		}
	})

	t.Run("high score easily overcomes distance", func(t *testing.T) {
		breakPoints := []BreakPoint{
			{Pos: 150, Score: 100, Type: "h1"},
			{Pos: 195, Score: 1, Type: "newline"},
		}
		cutoff := FindBestCutoff(breakPoints, 200, 100, 0.7, nil)
		if cutoff != 150 {
			t.Errorf("expected cutoff 150, got %d", cutoff)
		}
	})

	t.Run("skips break points inside code fences", func(t *testing.T) {
		breakPoints := []BreakPoint{
			{Pos: 150, Score: 100, Type: "h1"},
			{Pos: 180, Score: 20, Type: "blank"},
		}
		codeFences := []CodeFenceRegion{{Start: 140, End: 160}}
		cutoff := FindBestCutoff(breakPoints, 200, 100, 0.7, codeFences)
		if cutoff != 180 {
			t.Errorf("expected cutoff 180, got %d", cutoff)
		}
	})
}

func TestSmartChunkingIntegration(t *testing.T) {
	t.Run("chunkDocument prefers headings over arbitrary breaks", func(t *testing.T) {
		section1 := strings.Repeat("Introduction text here. ", 70) // ~1680 chars
		section2 := strings.Repeat("Main content text here. ", 50) // ~1150 chars
		content := section1 + "\n# Main Section\n" + section2

		bps := ScanBreakPoints(content)
		fences := FindCodeFences(content)

		chunks := ChunkDocumentWithBreakPoints(content, bps, fences, 2000, 0, 800)
		headingPos := strings.Index(content, "\n# Main Section")

		if len(chunks) < 2 {
			t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
		}
		if len(chunks[0].Text) != headingPos {
			t.Errorf("expected first chunk to end at heading position %d, got length %d", headingPos, len(chunks[0].Text))
		}
	})

	t.Run("chunkDocumentWithStrategy handles AST auto strategy", func(t *testing.T) {
		code := `package main
import "fmt"
func processData() {
	fmt.Println("processing")
}
`
		chunks := ChunkDocumentWithStrategy(code, "main.go", "auto")
		if len(chunks) == 0 {
			t.Fatal("expected chunks, got 0")
		}
	})
}
