package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Compile binary
	tempDir, err := os.MkdirTemp("", "qmd-cli-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	binaryPath = filepath.Join(tempDir, "qmd")
	cmd := exec.Command("go", "build", "-tags", "sqlite_fts5", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		panic(err)
	}

	code := m.Run()
	os.Exit(code)
}

func runCLI(t *testing.T, args []string, configDir string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if configDir != "" {
		cmd.Env = append(os.Environ(), "QMD_CONFIG_DIR="+configDir)
	}
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestCLIIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "qmd-config-")
	if err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test init
	t.Run("init command", func(t *testing.T) {
		// Run init in a temp project directory (not home)
		projDir, err := os.MkdirTemp("", "qmd-project-")
		if err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}
		defer os.RemoveAll(projDir)

		oldCwd, _ := os.Getwd()
		os.Chdir(projDir)
		defer os.Chdir(oldCwd)

		stdout, stderr, err := runCLI(t, []string{"init"}, projDir)
		if err != nil {
			t.Fatalf("init failed: %v (stderr: %q)", err, stderr)
		}
		if !strings.Contains(stdout, "ready to go with new local index") {
			t.Errorf("unexpected stdout: %q", stdout)
		}

		if _, err := os.Stat(filepath.Join(projDir, ".qmd", "index.yaml")); os.IsNotExist(err) {
			t.Error("index.yaml not created")
		}
		if _, err := os.Stat(filepath.Join(projDir, ".qmd", "index.sqlite")); os.IsNotExist(err) {
			t.Error("index.sqlite not created")
		}
	})

	// Test doctor
	t.Run("doctor command", func(t *testing.T) {
		stdout, _, err := runCLI(t, []string{"doctor"}, tempDir)
		if err != nil {
			t.Fatalf("doctor failed: %v", err)
		}
		if !strings.Contains(stdout, "healthy") {
			t.Errorf("expected doctor check to report healthy, got %q", stdout)
		}
	})

	// Test collection management and search
	t.Run("collection add, list, search, get", func(t *testing.T) {
		// Create a mock document folder to index
		docsDir, err := os.MkdirTemp("", "qmd-docs-")
		if err != nil {
			t.Fatalf("failed to create docs dir: %v", err)
		}
		defer os.RemoveAll(docsDir)

		file1 := filepath.Join(docsDir, "meeting.md")
		err = os.WriteFile(file1, []byte("# Project Sync\n\nDiscussion about quantum mechanics and general relativity.\n"), 0644)
		if err != nil {
			t.Fatalf("failed to write meeting.md: %v", err)
		}

		// Add collection
		stdout, stderr, err := runCLI(t, []string{"collection", "add", docsDir, "--name", "testcoll"}, tempDir)
		if err != nil {
			t.Fatalf("collection add failed: %v (stderr: %q)", err, stderr)
		}
		if !strings.Contains(stdout, "created successfully") {
			t.Errorf("unexpected collection add output: %q", stdout)
		}

		// List collections
		stdout, _, err = runCLI(t, []string{"collection", "list"}, tempDir)
		if err != nil {
			t.Fatalf("collection list failed: %v", err)
		}
		if !strings.Contains(stdout, "testcoll:") {
			t.Errorf("expected collection testcoll, got %q", stdout)
		}

		// Search FTS
		stdout, _, err = runCLI(t, []string{"search", "quantum"}, tempDir)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if !strings.Contains(stdout, "Project Sync") {
			t.Errorf("expected search result to contain 'Project Sync', got %q", stdout)
		}

		// Get document
		stdout, _, err = runCLI(t, []string{"get", "testcoll/meeting.md", "--md"}, tempDir)
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !strings.Contains(stdout, "quantum mechanics") {
			t.Errorf("expected document body, got %q", stdout)
		}

		// List files
		stdout, _, err = runCLI(t, []string{"ls", "testcoll"}, tempDir)
		if err != nil {
			t.Fatalf("ls failed: %v", err)
		}
		if !strings.Contains(stdout, "qmd://testcoll/meeting.md") {
			t.Errorf("expected ls to list meeting.md, got %q", stdout)
		}

		// Context add & list
		stdout, _, err = runCLI(t, []string{"context", "add", "/", "Global Context Text"}, tempDir)
		if err != nil {
			t.Fatalf("context add failed: %v", err)
		}
		stdout, _, err = runCLI(t, []string{"context", "list"}, tempDir)
		if err != nil {
			t.Fatalf("context list failed: %v", err)
		}
		if !strings.Contains(stdout, "Global Context Text") {
			t.Errorf("expected context to contain Global Context Text, got %q", stdout)
		}

		// Cleanup database
		stdout, _, err = runCLI(t, []string{"cleanup"}, tempDir)
		if err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
		if !strings.Contains(stdout, "cleaned successfully") {
			t.Errorf("unexpected cleanup output: %q", stdout)
		}
	})
}
