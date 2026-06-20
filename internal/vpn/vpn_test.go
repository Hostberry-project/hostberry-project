package vpn

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadRedactedConfigMetadataMissingFile(t *testing.T) {
	result := readRedactedConfigMetadata(filepath.Join(t.TempDir(), "missing.conf"), "TestVPN")

	if success, _ := result["success"].(bool); !success {
		t.Fatal("expected success for missing config metadata lookup")
	}
	if exists, _ := result["exists"].(bool); exists {
		t.Fatal("expected missing config to report exists=false")
	}
	if redacted, _ := result["redacted"].(bool); !redacted {
		t.Fatal("expected metadata response to be marked redacted")
	}
	if _, ok := result["config"]; ok {
		t.Fatal("config content must not be returned")
	}
}

func TestReadRedactedConfigMetadataExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.conf")
	content := []byte("secret-config")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	result := readRedactedConfigMetadata(path, "TestVPN")

	if success, _ := result["success"].(bool); !success {
		t.Fatal("expected success for existing config metadata lookup")
	}
	if exists, _ := result["exists"].(bool); !exists {
		t.Fatal("expected existing config to report exists=true")
	}
	if size, _ := result["size"].(int64); size != int64(len(content)) {
		t.Fatalf("unexpected size: got %d want %d", size, len(content))
	}
	if _, ok := result["config"]; ok {
		t.Fatal("config content must not be returned")
	}
}
