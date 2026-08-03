package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/acesaro/qmd/config"
)

type MessageData struct {
	Role string
	Text string
	Time time.Time
}

type SessionData struct {
	Harness  string
	ID       string
	Project  string
	Date     time.Time
	Messages []MessageData
}

func (s *SessionData) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Chat Session: %s\n\n", s.ID))
	sb.WriteString(fmt.Sprintf("- **Harness:** %s\n", s.Harness))
	sb.WriteString(fmt.Sprintf("- **Project:** %s\n", s.Project))
	if !s.Date.IsZero() {
		sb.WriteString(fmt.Sprintf("- **Date:** %s\n", s.Date.Format(time.RFC3339)))
	}
	sb.WriteString("\n## Messages\n")
	for _, msg := range s.Messages {
		roleName := "User"
		if msg.Role == "assistant" {
			roleName = "Assistant"
		} else if msg.Role == "tool" {
			roleName = "Tool"
		} else if msg.Role != "" {
			roleName = strings.Title(msg.Role)
		}
		sb.WriteString(fmt.Sprintf("\n### %s\n", roleName))
		if !msg.Time.IsZero() {
			sb.WriteString(fmt.Sprintf("*(Time: %s)*\n\n", msg.Time.Format(time.RFC3339)))
		}
		sb.WriteString(msg.Text)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ─── Path & Root Discovery ──────────────────────────────────────────────────

func GetClaudeRoot() string {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return filepath.Join(v, "projects")
	}
	return filepath.Join(config.QmdHomedir(), ".claude", "projects")
}

func GetPiRoot() string {
	if v := os.Getenv("DEJA_PI_ROOT"); v != "" {
		return v
	}
	return filepath.Join(config.QmdHomedir(), ".pi", "agent", "sessions")
}

func GetCopilotRoot() string {
	if v := os.Getenv("DEJA_COPILOT_ROOT"); v != "" {
		return v
	}
	return filepath.Join(config.QmdHomedir(), ".copilot", "session-state")
}

func GetAntigravityRoots() []string {
	if v := os.Getenv("DEJA_ANTIGRAVITY_ROOT"); v != "" {
		return []string{v}
	}
	roots, err := filepath.Glob(filepath.Join(config.QmdHomedir(), ".gemini", "antigravity*"))
	if err != nil {
		return nil
	}
	var out []string
	for _, root := range roots {
		if fi, err := os.Stat(root); err == nil && fi.IsDir() {
			out = append(out, root)
		}
	}
	return out
}

// ─── Ingestion entry point ──────────────────────────────────────────────────

func DiscoverAndSyncHarnesses() error {
	memoryDir := filepath.Join(config.GetConfigDir(), "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return err
	}

	// 1. Claude
	claudeRoot := GetClaudeRoot()
	if fi, err := os.Stat(claudeRoot); err == nil && fi.IsDir() {
		if err := syncClaude(claudeRoot, filepath.Join(memoryDir, "claude")); err != nil {
			fmt.Fprintf(os.Stderr, "Error syncing Claude history: %v\n", err)
		}
	}

	// 2. Pi
	piRoot := GetPiRoot()
	if fi, err := os.Stat(piRoot); err == nil && fi.IsDir() {
		if err := syncPi(piRoot, filepath.Join(memoryDir, "pi")); err != nil {
			fmt.Fprintf(os.Stderr, "Error syncing pi history: %v\n", err)
		}
	}

	// 3. Copilot
	copilotRoot := GetCopilotRoot()
	if fi, err := os.Stat(copilotRoot); err == nil && fi.IsDir() {
		if err := syncCopilot(copilotRoot, filepath.Join(memoryDir, "copilot")); err != nil {
			fmt.Fprintf(os.Stderr, "Error syncing Copilot history: %v\n", err)
		}
	}

	// 4. Antigravity
	antigravityRoots := GetAntigravityRoots()
	if len(antigravityRoots) > 0 {
		if err := syncAntigravity(antigravityRoots, filepath.Join(memoryDir, "antigravity")); err != nil {
			fmt.Fprintf(os.Stderr, "Error syncing Antigravity history: %v\n", err)
		}
	}

	return nil
}

// ─── Sync Pipelines ─────────────────────────────────────────────────────────

func syncClaude(root string, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	var sessionsFound = make(map[string]bool)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"subagents"+string(filepath.Separator)) {
			return nil
		}

		sessionID := strings.TrimSuffix(info.Name(), ".jsonl")
		sessionsFound[sessionID] = true

		destPath := filepath.Join(destDir, sessionID+".md")
		destStat, destErr := os.Stat(destPath)
		if destErr == nil && destStat.ModTime().After(info.ModTime()) {
			return nil
		}

		session, err := parseClaudeSession(path)
		if err != nil || session == nil {
			return nil
		}

		md := session.ToMarkdown()
		_ = os.WriteFile(destPath, []byte(md), 0644)
		return nil
	})

	if err != nil {
		return err
	}

	cleanupOrphanedMarkdown(destDir, sessionsFound)

	files, _ := os.ReadDir(destDir)
	if len(files) > 0 {
		ensureCollectionRegistered("claude", destDir)
	}

	return nil
}

func syncPi(root string, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	var sessionsFound = make(map[string]bool)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") {
			return nil
		}

		sessionID := strings.TrimSuffix(info.Name(), ".jsonl")
		sessionsFound[sessionID] = true

		destPath := filepath.Join(destDir, sessionID+".md")
		destStat, destErr := os.Stat(destPath)
		if destErr == nil && destStat.ModTime().After(info.ModTime()) {
			return nil
		}

		session, err := parsePiSession(path)
		if err != nil || session == nil {
			return nil
		}

		md := session.ToMarkdown()
		_ = os.WriteFile(destPath, []byte(md), 0644)
		return nil
	})

	if err != nil {
		return err
	}

	cleanupOrphanedMarkdown(destDir, sessionsFound)

	files, _ := os.ReadDir(destDir)
	if len(files) > 0 {
		ensureCollectionRegistered("pi", destDir)
	}

	return nil
}

func syncCopilot(root string, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	var sessionsFound = make(map[string]bool)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != "events.jsonl" {
			return nil
		}

		sessionID := filepath.Base(filepath.Dir(path))
		sessionsFound[sessionID] = true

		destPath := filepath.Join(destDir, sessionID+".md")
		destStat, destErr := os.Stat(destPath)
		if destErr == nil && destStat.ModTime().After(info.ModTime()) {
			return nil
		}

		session, err := parseCopilotSession(path)
		if err != nil || session == nil {
			return nil
		}

		md := session.ToMarkdown()
		_ = os.WriteFile(destPath, []byte(md), 0644)
		return nil
	})

	if err != nil {
		return err
	}

	cleanupOrphanedMarkdown(destDir, sessionsFound)

	files, _ := os.ReadDir(destDir)
	if len(files) > 0 {
		ensureCollectionRegistered("copilot", destDir)
	}

	return nil
}

func syncAntigravity(roots []string, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	var sessionsFound = make(map[string]bool)
	for _, root := range roots {
		brainDir := filepath.Join(root, "brain")
		entries, err := os.ReadDir(brainDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			sessionID := entry.Name()
			transcriptPath := filepath.Join(brainDir, sessionID, ".system_generated", "logs", "transcript.jsonl")
			info, err := os.Stat(transcriptPath)
			if err != nil {
				continue
			}

			sessionsFound[sessionID] = true

			destPath := filepath.Join(destDir, sessionID+".md")
			destStat, destErr := os.Stat(destPath)
			if destErr == nil && destStat.ModTime().After(info.ModTime()) {
				continue
			}

			session, err := parseAntigravitySession(transcriptPath, root)
			if err != nil || session == nil {
				continue
			}

			md := session.ToMarkdown()
			_ = os.WriteFile(destPath, []byte(md), 0644)
		}
	}

	cleanupOrphanedMarkdown(destDir, sessionsFound)

	files, _ := os.ReadDir(destDir)
	if len(files) > 0 {
		ensureCollectionRegistered("antigravity", destDir)
	}

	return nil
}

func cleanupOrphanedMarkdown(destDir string, activeSessions map[string]bool) {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		sessionID := strings.TrimSuffix(name, ".md")
		if !activeSessions[sessionID] {
			_ = os.Remove(filepath.Join(destDir, name))
		}
	}
}

func ensureCollectionRegistered(name, path string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}
	if _, exists := cfg.Collections[name]; !exists {
		coll := config.Collection{
			Path:    path,
			Pattern: "**/*.md",
			Context: map[string]string{
				"/": fmt.Sprintf("Memory and chat history from %s sessions", getHarnessDisplayName(name)),
			},
		}
		cfg.Collections[name] = coll
		err = config.SaveConfig(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to automatically register collection %q: %v\n", name, err)
		} else {
			fmt.Printf("Registered new GenAI memory collection: %s -> %s\n", name, path)
		}
	}
}

func getHarnessDisplayName(name string) string {
	switch name {
	case "claude":
		return "Claude Code"
	case "pi":
		return "pi coding agent"
	case "copilot":
		return "Copilot CLI"
	case "antigravity":
		return "Antigravity CLI"
	default:
		return name
	}
}

// ─── Session Parsers ────────────────────────────────────────────────────────

type rawClaudeLine struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionId"`
	Timestamp json.RawMessage `json:"timestamp"`
	Message   *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func parseClaudeSession(path string) (*SessionData, error) {
	session := &SessionData{
		Harness: "claude",
		ID:      strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		Project: decodeClaudeProjectName(filepath.Base(filepath.Dir(path))),
	}

	err := scanJSONLFile(path, func(line []byte) error {
		var v rawClaudeLine
		if err := json.Unmarshal(line, &v); err != nil {
			return nil
		}
		if v.Type != "user" && v.Type != "assistant" {
			return nil
		}
		if v.SessionID != "" {
			session.ID = v.SessionID
		}
		t := parseTimestamp(v.Timestamp)
		if session.Date.IsZero() || (!t.IsZero() && t.Before(session.Date)) {
			session.Date = t
		}
		role := v.Type
		text := ""
		if v.Message != nil {
			if v.Message.Role != "" {
				role = v.Message.Role
			}
			text = extractTextFromContent(v.Message.Content)
		}
		if text != "" {
			session.Messages = append(session.Messages, MessageData{Role: role, Text: text, Time: t})
		}
		return nil
	})

	if len(session.Messages) == 0 {
		return nil, nil
	}
	return session, err
}

type rawPiLine struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Cwd       string          `json:"cwd"`
	Timestamp json.RawMessage `json:"timestamp"`
	Message   *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func parsePiSession(path string) (*SessionData, error) {
	session := &SessionData{
		Harness: "pi",
		ID:      strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		Project: decodeClaudeProjectName(filepath.Base(filepath.Dir(path))),
	}

	err := scanJSONLFile(path, func(line []byte) error {
		var v rawPiLine
		if err := json.Unmarshal(line, &v); err != nil {
			return nil
		}
		if v.Type == "session" {
			if v.ID != "" {
				session.ID = v.ID
			}
			if v.Cwd != "" {
				session.Project = decodeClaudeProjectName(filepath.Base(v.Cwd))
			}
			t := parseTimestamp(v.Timestamp)
			if session.Date.IsZero() || (!t.IsZero() && t.Before(session.Date)) {
				session.Date = t
			}
		} else if v.Type == "message" && v.Message != nil {
			role := v.Message.Role
			if role != "user" && role != "assistant" {
				return nil
			}
			t := parseTimestamp(v.Timestamp)
			text := extractTextFromContent(v.Message.Content)
			if text != "" {
				session.Messages = append(session.Messages, MessageData{Role: role, Text: text, Time: t})
			}
		}
		return nil
	})

	if len(session.Messages) == 0 {
		return nil, nil
	}
	return session, err
}

type rawCopilotLine struct {
	Type      string          `json:"type"`
	Timestamp json.RawMessage `json:"timestamp"`
	Data      map[string]any  `json:"data"`
}

func parseCopilotSession(path string) (*SessionData, error) {
	session := &SessionData{
		Harness: "copilot",
		ID:      filepath.Base(filepath.Dir(path)),
		Project: "-",
	}

	err := scanJSONLFile(path, func(line []byte) error {
		var v rawCopilotLine
		if err := json.Unmarshal(line, &v); err != nil {
			return nil
		}
		t := parseTimestamp(v.Timestamp)
		switch v.Type {
		case "session.start":
			if v.Data == nil {
				return nil
			}
			if id, ok := v.Data["sessionId"].(string); ok && id != "" {
				session.ID = id
			}
			if startTime, ok := v.Data["startTime"]; ok {
				var stBytes []byte
				if stStr, ok := startTime.(string); ok {
					stBytes, _ = json.Marshal(stStr)
				} else if stNum, ok := startTime.(float64); ok {
					stBytes, _ = json.Marshal(stNum)
				}
				if len(stBytes) > 0 {
					session.Date = parseTimestamp(stBytes)
				}
			}
			if ctx, ok := v.Data["context"].(map[string]any); ok {
				if cwd, ok := ctx["cwd"].(string); ok && cwd != "" {
					session.Project = copilotProjectName(cwd)
				}
			}
		case "user.message", "assistant.message":
			if v.Data == nil {
				return nil
			}
			role := "user"
			if v.Type == "assistant.message" {
				role = "assistant"
			}
			if session.Date.IsZero() && !t.IsZero() {
				session.Date = t
			}
			if content, ok := v.Data["content"].(string); ok && content != "" {
				session.Messages = append(session.Messages, MessageData{Role: role, Text: content, Time: t})
			}
		}
		return nil
	})

	if len(session.Messages) == 0 {
		return nil, nil
	}
	return session, err
}

type rawAntigravityLine struct {
	Source    string          `json:"source"`
	Type      string          `json:"type"`
	Content   string          `json:"content"`
	CreatedAt json.RawMessage `json:"created_at"`
}

func parseAntigravitySession(path string, root string) (*SessionData, error) {
	id := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	session := &SessionData{
		Harness: "antigravity",
		ID:      id,
		Project: antigravityProjectName(id, root),
	}

	err := scanJSONLFile(path, func(line []byte) error {
		var v rawAntigravityLine
		if err := json.Unmarshal(line, &v); err != nil {
			return nil
		}
		t := parseTimestamp(v.CreatedAt)
		if session.Date.IsZero() && !t.IsZero() {
			session.Date = t
		}
		var role string
		switch v.Source {
		case "USER_EXPLICIT":
			role = "user"
		case "MODEL":
			role = "assistant"
		default:
			return nil
		}

		text := v.Content
		if role == "user" {
			text = cleanAntigravityUserContent(text)
		} else if role == "assistant" {
			text = cleanAntigravityAssistantContent(v.Type, text)
		}

		if text != "" {
			session.Messages = append(session.Messages, MessageData{Role: role, Text: text, Time: t})
		}
		return nil
	})

	if len(session.Messages) == 0 {
		return nil, nil
	}
	return session, err
}

// ─── Helper Parsers ─────────────────────────────────────────────────────────

func scanJSONLFile(path string, fn func(line []byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 1024*1024)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			if err := fn(line); err != nil {
				return err
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func extractTextFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		var parts []string
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				if txt, ok := m["text"].(string); ok && txt != "" {
					parts = append(parts, txt)
				} else if content, ok := m["content"].(string); ok && content != "" {
					parts = append(parts, content)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func parseTimestamp(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if t, err := time.Parse(time.RFC3339, str); err == nil {
			return t.UTC()
		}
		if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
			return t.UTC()
		}
	}
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		n := int64(num)
		if n > 1e12 {
			return time.UnixMilli(n).UTC()
		}
		if n > 0 {
			return time.Unix(n, 0).UTC()
		}
	}
	return time.Time{}
}

func decodeClaudeProjectName(dirName string) string {
	trimmed := strings.Trim(dirName, "-")
	if trimmed == "" {
		return "-"
	}
	parts := strings.Split(trimmed, "-")
	var clean []string
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 {
		return dirName
	}
	if len(clean) == 1 {
		return clean[0]
	}
	return filepath.Join(clean[len(clean)-2], clean[len(clean)-1])
}

func copilotProjectName(cwd string) string {
	cwd = strings.TrimRight(cwd, "/\\")
	base := filepath.Base(cwd)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return ""
	}
	parent := filepath.Base(filepath.Dir(cwd))
	if parent != "" && parent != "." && parent != string(filepath.Separator) && !strings.Contains(parent, ":") {
		return parent + "/" + base
	}
	return base
}

func antigravityProjectName(id string, root string) string {
	p := filepath.Join(root, "cache", "conversation_metadata.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return "-"
	}
	var doc struct {
		Conversations map[string]struct {
			Summary struct {
				WorkspaceURIs []string `json:"WorkspaceURIs"`
			} `json:"summary"`
		} `json:"conversations"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return "-"
	}
	for key, c := range doc.Conversations {
		if !strings.HasPrefix(key, id) && !strings.HasPrefix(id, key) {
			continue
		}
		for _, uri := range c.Summary.WorkspaceURIs {
			if w, ok := strings.CutPrefix(uri, "file://"); ok && w != "" {
				return filepath.Base(w)
			}
		}
	}
	return "-"
}

func cleanAntigravityUserContent(text string) string {
	var userBlockREs = []*regexp.Regexp{
		regexp.MustCompile(`(?s)<ADDITIONAL_METADATA>.*?</ADDITIONAL_METADATA>`),
		regexp.MustCompile(`(?s)<USER_SETTINGS_CHANGE>.*?</USER_SETTINGS_CHANGE>`),
	}
	for _, re := range userBlockREs {
		text = re.ReplaceAllString(text, "")
	}
	text = regexp.MustCompile(`^\s*<USER_REQUEST>\s*`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\s*</USER_REQUEST>\s*$`).ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func cleanAntigravityAssistantContent(kind string, text string) string {
	if kind == "PLANNER_RESPONSE" || kind == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	cut := 0
	for cut < len(lines) {
		l := strings.TrimSpace(lines[cut])
		if l == "" || strings.HasPrefix(l, "Created At:") || strings.HasPrefix(l, "Completed At:") {
			cut++
			continue
		}
		break
	}
	body := strings.TrimSpace(strings.Join(lines[cut:], "\n"))
	if body == "" {
		return ""
	}
	return fmt.Sprintf("*(Executed %s)*\n\n%s", kind, body)
}

