package common

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileReadTool(t *testing.T) {
	tool := NewFileReadTool()

	if tool.Name() != "file_read" {
		t.Errorf("expected name 'file_read', got %q", tool.Name())
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, CTF!"
	os.WriteFile(testFile, []byte(testContent), 0644)

	input, _ := json.Marshal(map[string]string{"path": testFile})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != testContent {
		t.Errorf("expected %q, got %q", testContent, result)
	}
}

func TestFileReadToolNotFound(t *testing.T) {
	tool := NewFileReadTool()

	input, _ := json.Marshal(map[string]string{"path": "/nonexistent/file.txt"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
