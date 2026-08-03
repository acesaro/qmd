package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseClaudeSession(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "qmd-claude-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write mock Claude jsonl file
	jsonlContent := `{"type":"user","sessionId":"sess1","timestamp":"2026-01-02T03:04:05Z","message":{"role":"user","content":"Hello world"}}
{"type":"assistant","sessionId":"sess1","timestamp":1767323050,"message":{"role":"assistant","content":[{"type":"text","text":"Hi there!"}]}}`

	filePath := filepath.Join(tempDir, "sess1.jsonl")
	if err := os.WriteFile(filePath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	session, err := parseClaudeSession(filePath)
	if err != nil {
		t.Fatalf("parseClaudeSession failed: %v", err)
	}

	if session == nil {
		t.Fatal("session is nil")
	}

	if session.ID != "sess1" {
		t.Errorf("expected session ID sess1, got %q", session.ID)
	}

	if len(session.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(session.Messages))
	}

	if session.Messages[0].Role != "user" || session.Messages[0].Text != "Hello world" {
		t.Errorf("unexpected message 0: %+v", session.Messages[0])
	}

	if session.Messages[1].Role != "assistant" || session.Messages[1].Text != "Hi there!" {
		t.Errorf("unexpected message 1: %+v", session.Messages[1])
	}
}

func TestParsePiSession(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "qmd-pi-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	jsonlContent := `{"type":"session","id":"pi-sess","cwd":"/path/to/my-project","timestamp":"2026-01-02T03:04:05Z"}
{"type":"message","timestamp":"2026-01-02T03:04:06Z","message":{"role":"user","content":"Run this"}}
{"type":"message","timestamp":"2026-01-02T03:04:07Z","message":{"role":"assistant","content":"Sure"}}`

	filePath := filepath.Join(tempDir, "pi-sess.jsonl")
	if err := os.WriteFile(filePath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	session, err := parsePiSession(filePath)
	if err != nil {
		t.Fatalf("parsePiSession failed: %v", err)
	}

	if session == nil {
		t.Fatal("session is nil")
	}

	if session.ID != "pi-sess" {
		t.Errorf("expected session ID pi-sess, got %q", session.ID)
	}

	if len(session.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(session.Messages))
	}

	if session.Messages[0].Role != "user" || session.Messages[0].Text != "Run this" {
		t.Errorf("unexpected message 0: %+v", session.Messages[0])
	}
}

func TestParseCopilotSession(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "qmd-copilot-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	jsonlContent := `{"type":"session.start","timestamp":"2026-01-02T03:04:05Z","data":{"sessionId":"copilot-sess","startTime":1767323045,"context":{"cwd":"/Users/acesaro/git/github.com/acesaro/qmd"}}}
{"type":"user.message","timestamp":"2026-01-02T03:04:06Z","data":{"content":"Help me please"}}
{"type":"assistant.message","timestamp":"2026-01-02T03:04:07Z","data":{"content":"I am helper"}}`

	dirPath := filepath.Join(tempDir, "copilot-sess")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	filePath := filepath.Join(dirPath, "events.jsonl")
	if err := os.WriteFile(filePath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	session, err := parseCopilotSession(filePath)
	if err != nil {
		t.Fatalf("parseCopilotSession failed: %v", err)
	}

	if session == nil {
		t.Fatal("session is nil")
	}

	if session.ID != "copilot-sess" {
		t.Errorf("expected session ID copilot-sess, got %q", session.ID)
	}

	if session.Project != "acesaro/qmd" {
		t.Errorf("expected project acesaro/qmd, got %q", session.Project)
	}

	if len(session.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(session.Messages))
	}

	if session.Messages[0].Role != "user" || session.Messages[0].Text != "Help me please" {
		t.Errorf("unexpected message 0: %+v", session.Messages[0])
	}
}

func TestParseAntigravitySession(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "qmd-antigravity-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	jsonlContent := `{"source":"USER_EXPLICIT","type":"USER_INPUT","content":"<USER_REQUEST>Test request</USER_REQUEST>","created_at":"2026-01-02T03:04:05Z"}
{"source":"MODEL","type":"PLANNER_RESPONSE","content":"planner content","created_at":"2026-01-02T03:04:06Z"}
{"source":"MODEL","type":"RUN_COMMAND","content":"Created At: 2026-01-02T03:04:07Z\nCompleted At: 2026-01-02T03:04:08Z\nCommand output","created_at":"2026-01-02T03:04:07Z"}`

	// Session folder layout: root/brain/<sessionID>/.system_generated/logs/transcript.jsonl
	sessDir := filepath.Join(tempDir, "brain", "anti-sess", ".system_generated", "logs")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	filePath := filepath.Join(sessDir, "transcript.jsonl")
	if err := os.WriteFile(filePath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	session, err := parseAntigravitySession(filePath, tempDir)
	if err != nil {
		t.Fatalf("parseAntigravitySession failed: %v", err)
	}

	if session == nil {
		t.Fatal("session is nil")
	}

	if session.ID != "anti-sess" {
		t.Errorf("expected session ID anti-sess, got %q", session.ID)
	}

	if len(session.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(session.Messages))
	}

	if session.Messages[0].Role != "user" || session.Messages[0].Text != "Test request" {
		t.Errorf("unexpected message 0: %+v", session.Messages[0])
	}

	if session.Messages[1].Role != "assistant" || session.Messages[1].Text != "planner content" {
		t.Errorf("unexpected message 1: %+v", session.Messages[1])
	}
}

func TestSessionToMarkdown(t *testing.T) {
	session := &SessionData{
		Harness: "claude",
		ID:      "test-sess-123",
		Project: "acesaro/qmd",
		Date:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Messages: []MessageData{
			{Role: "user", Text: "Hello assistant", Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
			{Role: "assistant", Text: "Hello user", Time: time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)},
		},
	}

	md := session.ToMarkdown()
	expected := `# Chat Session: test-sess-123

- **Harness:** claude
- **Project:** acesaro/qmd
- **Date:** 2026-01-02T03:04:05Z

## Messages

### User
*(Time: 2026-01-02T03:04:05Z)*

Hello assistant

### Assistant
*(Time: 2026-01-02T03:04:06Z)*

Hello user
`

	if md != expected {
		t.Errorf("unexpected markdown output:\n---GOT---\n%s\n---EXPECTED---\n%s\n", md, expected)
	}
}
