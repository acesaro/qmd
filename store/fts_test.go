package store

import (
	"testing"
)

func TestBuildFTS5Query(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"foo bar", `"foo"* AND "bar"*`},
		{"performance", `"performance"*`},
		{`"machine learning"`, `"machine learning"`},
		{`"C++ performance"`, `"c performance"`},
		{"performance -sports", `"performance"* NOT "sports"*`},
		{`performance -"sports athlete"`, `"performance"* NOT "sports athlete"`},
		{"performance -sports -athlete", `"performance"* NOT "sports"* NOT "athlete"*`},
		{`"machine learning" -sports -athlete`, `"machine learning" NOT "sports"* NOT "athlete"*`},
		{"-sports -athlete", ""},
		{"", ""},
		{"   ", ""},
		{"hello!world", `"helloworld"*`},
		{"multi-agent", `"multi agent"`},
		{"DEC-0054", `"dec 0054"`},
		{"gpt-4", `"gpt 4"`},
		{"foo-bar-baz", `"foo bar baz"`},
		{"multi-agent memory", `"multi agent" AND "memory"*`},
		{"multi-agent -sports", `"multi agent" NOT "sports"*`},
		{"performance -multi-agent", `"performance"* NOT "multi agent"`},
		{"2026.4.10", `"2026"* AND "4"* AND "10"*`},
		{"performance -2026.4.10", `"performance"* NOT ("2026"* AND "4"* AND "10"*)`},
	}

	for _, tc := range tests {
		got := BuildFTS5Query(tc.query)
		if got != tc.expected {
			t.Errorf("BuildFTS5Query(%q) = %q; want %q", tc.query, got, tc.expected)
		}
	}
}

func TestNormalizeCjkForFTS(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"日本語", " 日 本 語 "},
		{"Hello 日本語 World", "Hello  日 本 語  World"},
		{"한국어", " 한 국 어 "},
	}

	for _, tc := range tests {
		got := NormalizeCjkForFTS(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeCjkForFTS(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}
