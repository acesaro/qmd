package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/acesaro/qmd/store"
)

func createTestStore(t *testing.T) (*store.Store, string) {
	tempDir, err := os.MkdirTemp("", "qmd-mcp-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.sqlite")
	s, err := store.OpenStore(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open store: %v", err)
	}

	return s, tempDir
}

func TestMcpInitializeAndTools(t *testing.T) {
	s, tempDir := createTestStore(t)
	defer s.Close()
	defer os.RemoveAll(tempDir)

	// Test Initialize
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := handleRequest(s, req)
	if resp.Error != nil {
		t.Fatalf("initialize failed: %s", resp.Error.Message)
	}

	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected initialize response: %+v", resp.Result)
	}
	if resultMap["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %v", resultMap["protocolVersion"])
	}

	// Test tools/list
	reqList := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}
	respList := handleRequest(s, reqList)
	if respList.Error != nil {
		t.Fatalf("tools/list failed: %s", respList.Error.Message)
	}
}

func TestMcpToolsCallStatus(t *testing.T) {
	s, tempDir := createTestStore(t)
	defer s.Close()
	defer os.RemoveAll(tempDir)

	callParams := struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}{
		Name:      "status",
		Arguments: json.RawMessage(`{}`),
	}

	paramsData, _ := json.Marshal(callParams)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  paramsData,
	}

	resp := handleRequest(s, req)
	if resp.Error != nil {
		t.Fatalf("tools/call status failed: %s", resp.Error.Message)
	}

	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected response result: %+v", resp.Result)
	}

	content, ok := resultMap["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("expected content, got: %+v", resultMap["content"])
	}

	textMap := content[0].(map[string]interface{})
	textVal := textMap["text"].(string)
	if !strings.Contains(textVal, "QMD Index Status:") {
		t.Errorf("expected summary, got: %q", textVal)
	}
}
