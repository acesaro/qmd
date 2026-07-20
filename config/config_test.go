package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGetConfigPathDefaults(t *testing.T) {
	origConfigDir := os.Getenv("QMD_CONFIG_DIR")
	origXdg := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("QMD_CONFIG_DIR")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if origConfigDir != "" {
			os.Setenv("QMD_CONFIG_DIR", origConfigDir)
		}
		if origXdg != "" {
			os.Setenv("XDG_CONFIG_HOME", origXdg)
		}
	}()

	ResetConfigSource()
	SetConfigIndexName("index")

	expected := filepath.Join(QmdHomedir(), ".config", "qmd", "index.yml")
	if got := GetConfigFilePath(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestGetConfigPathUSERPROFILE(t *testing.T) {
	origHome := os.Getenv("HOME")
	origConfigDir := os.Getenv("QMD_CONFIG_DIR")
	origXdg := os.Getenv("XDG_CONFIG_HOME")
	origUP := os.Getenv("USERPROFILE")

	os.Unsetenv("HOME")
	os.Unsetenv("QMD_CONFIG_DIR")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Setenv("USERPROFILE", "/Users/windows-user")

	defer func() {
		if origHome != "" {
			os.Setenv("HOME", origHome)
		} else {
			os.Unsetenv("HOME")
		}
		if origConfigDir != "" {
			os.Setenv("QMD_CONFIG_DIR", origConfigDir)
		}
		if origXdg != "" {
			os.Setenv("XDG_CONFIG_HOME", origXdg)
		}
		if origUP != "" {
			os.Setenv("USERPROFILE", origUP)
		} else {
			os.Unsetenv("USERPROFILE")
		}
	}()

	ResetConfigSource()
	SetConfigIndexName("index")

	expected := filepath.Join("/Users/windows-user", ".config", "qmd", "index.yml")
	if got := GetConfigFilePath(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConfigDirPriority(t *testing.T) {
	os.Setenv("QMD_CONFIG_DIR", "/custom/qmd-config")
	os.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	defer func() {
		os.Unsetenv("QMD_CONFIG_DIR")
		os.Unsetenv("XDG_CONFIG_HOME")
	}()

	expected := filepath.Join("/custom/qmd-config", "index.yml")
	if got := GetConfigFilePath(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestXdgConfigHome(t *testing.T) {
	os.Unsetenv("QMD_CONFIG_DIR")
	os.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	defer func() {
		os.Unsetenv("XDG_CONFIG_HOME")
	}()

	expected := filepath.Join("/xdg/config", "qmd", "index.yml")
	if got := GetConfigFilePath(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestCustomIndexName(t *testing.T) {
	os.Unsetenv("QMD_CONFIG_DIR")
	os.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	defer func() {
		os.Unsetenv("XDG_CONFIG_HOME")
	}()

	SetConfigIndexName("myindex")
	expected := filepath.Join("/xdg/config", "qmd", "myindex.yml")
	if got := GetConfigFilePath(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestLoadEmptyConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "qmd-empty-config-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("QMD_CONFIG_DIR", tempDir)
	defer os.Unsetenv("QMD_CONFIG_DIR")

	configPath := filepath.Join(tempDir, "index.yml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty config: %v", err)
	}

	conf, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(conf.Collections) != 0 {
		t.Errorf("expected empty collections, got %v", conf.Collections)
	}
}

func TestLocalConfigDiscovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "qmd-local-config-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	qmdDir := filepath.Join(tempDir, ".qmd")
	if err := os.MkdirAll(qmdDir, 0755); err != nil {
		t.Fatalf("failed to create .qmd dir: %v", err)
	}

	yamlPath := filepath.Join(qmdDir, "index.yaml")
	if err := os.WriteFile(yamlPath, []byte("collections: {}\n"), 0644); err != nil {
		t.Fatalf("failed to write index.yaml: %v", err)
	}

	nested := filepath.Join(tempDir, "wiki", "Shopify")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	found, err := FindLocalConfigPath(nested)
	if err != nil {
		t.Fatalf("FindLocalConfigPath failed: %v", err)
	}

	expectedFound, _ := filepath.Abs(yamlPath)
	if found != expectedFound {
		t.Errorf("expected local config path %q, got %q", expectedFound, found)
	}

	dbPath := GetLocalDbPath(found)
	expectedDb := filepath.Join(qmdDir, "index.sqlite")
	if dbPath != expectedDb {
		t.Errorf("expected local db path %q, got %q", expectedDb, dbPath)
	}
}

func TestLoadSaveCollections(t *testing.T) {
	var conf CollectionConfig
	SetConfigSourceInline(&conf)
	defer ResetConfigSource()

	err := AddCollection("testcol", "/some/path", "*.md")
	if err != nil {
		t.Fatalf("AddCollection failed: %v", err)
	}

	got, err := GetCollection("testcol")
	if err != nil {
		t.Fatalf("GetCollection failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected collection, got nil")
	}
	if got.Path != "/some/path" || got.Pattern != "*.md" {
		t.Errorf("unexpected collection settings: %+v", got)
	}

	list, err := ListCollections()
	if err != nil {
		t.Fatalf("ListCollections failed: %v", err)
	}
	if len(list) != 1 || list[0].Name != "testcol" {
		t.Errorf("unexpected list: %+v", list)
	}

	removed, err := RemoveCollection("testcol")
	if err != nil {
		t.Fatalf("RemoveCollection failed: %v", err)
	}
	if !removed {
		t.Error("expected RemoveCollection to return true")
	}

	list2, err := ListCollections()
	if err != nil {
		t.Fatalf("ListCollections failed: %v", err)
	}
	if len(list2) != 0 {
		t.Errorf("expected empty list, got %+v", list2)
	}
}

func TestContextManagement(t *testing.T) {
	var conf CollectionConfig
	SetConfigSourceInline(&conf)
	defer ResetConfigSource()

	err := AddCollection("testcol", "/some/path", "*.md")
	if err != nil {
		t.Fatalf("AddCollection failed: %v", err)
	}

	ok, err := AddContext("testcol", "/sub", "Subfolder context")
	if err != nil || !ok {
		t.Fatalf("AddContext failed: %v, %v", err, ok)
	}

	ok, err = AddContext("testcol", "/", "Root context")
	if err != nil || !ok {
		t.Fatalf("AddContext failed: %v, %v", err, ok)
	}

	ctx, err := FindContextForPath("testcol", "sub/file.md")
	if err != nil {
		t.Fatalf("FindContextForPath failed: %v", err)
	}
	if ctx != "Subfolder context" {
		t.Errorf("expected 'Subfolder context', got %q", ctx)
	}

	ctx, err = FindContextForPath("testcol", "other/file.md")
	if err != nil {
		t.Fatalf("FindContextForPath failed: %v", err)
	}
	if ctx != "Root context" {
		t.Errorf("expected 'Root context', got %q", ctx)
	}

	records, err := ListAllContexts()
	if err != nil {
		t.Fatalf("ListAllContexts failed: %v", err)
	}
	expectedRecords := []ContextRecord{
		{Collection: "testcol", Path: "/", Context: "Root context"},
		{Collection: "testcol", Path: "/sub", Context: "Subfolder context"},
	}
	if !reflect.DeepEqual(records, expectedRecords) {
		t.Errorf("expected records %+v, got %+v", expectedRecords, records)
	}
}
