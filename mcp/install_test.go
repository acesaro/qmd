package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateMcpConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mcp_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")

	// 1. Test update when file doesn't exist
	err = updateMcpConfig(configPath, "/usr/local/bin/qmd")
	if err != nil {
		t.Fatalf("expected no error updating non-existent config, got %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcpServers not found in generated config")
	}

	qmdServer, ok := mcpServers["qmd"].(map[string]interface{})
	if !ok {
		t.Fatal("qmd server configuration not found in generated config")
	}

	if qmdServer["command"] != "/usr/local/bin/qmd" {
		t.Errorf("expected command '/usr/local/bin/qmd', got '%v'", qmdServer["command"])
	}

	// 2. Test updating when file already exists with other content
	existingConfig := map[string]interface{}{
		"allowNonWorkspaceAccess": true,
		"mcpServers": map[string]interface{}{
			"other-server": map[string]interface{}{
				"command": "other",
			},
		},
	}
	existingData, _ := json.Marshal(existingConfig)
	_ = os.WriteFile(configPath, existingData, 0644)

	err = updateMcpConfig(configPath, "/usr/local/bin/qmd-new")
	if err != nil {
		t.Fatal(err)
	}

	newData, _ := os.ReadFile(configPath)
	var newConfig map[string]interface{}
	_ = json.Unmarshal(newData, &newConfig)

	if newConfig["allowNonWorkspaceAccess"] != true {
		t.Error("lost existing config keys during update")
	}

	newMcpServers, _ := newConfig["mcpServers"].(map[string]interface{})
	if _, exists := newMcpServers["other-server"]; !exists {
		t.Error("lost existing other mcp server config")
	}

	qmdNewServer, ok := newMcpServers["qmd"].(map[string]interface{})
	if !ok {
		t.Fatal("lost qmd server config")
	}

	if qmdNewServer["command"] != "/usr/local/bin/qmd-new" {
		t.Errorf("failed to update qmd server path, got '%v'", qmdNewServer["command"])
	}
}
