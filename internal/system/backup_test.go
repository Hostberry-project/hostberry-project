package system

import (
	"os"
	"path/filepath"
	"testing"

	"hostberry/internal/config"
)

func TestCreateAndRestoreBackup(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "data", "hostberry.db")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0750)
	_ = os.WriteFile(cfgPath, []byte("server:\n  port: 8000\n"), 0644)
	_ = os.WriteFile(dbPath, []byte("sqlite-test"), 0600)

	config.AppConfig = &config.Config{
		Database: config.DatabaseConfig{Type: "sqlite", Path: dbPath},
	}

	oldWd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	path, err := CreateSystemBackup()
	if err != nil {
		t.Fatalf("CreateSystemBackup: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	_ = os.WriteFile(cfgPath, []byte("corrupted"), 0644)
	name := filepath.Base(path)
	if err := RestoreSystemBackup(name); err != nil {
		t.Fatalf("RestoreSystemBackup: %v", err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil || string(data) != "server:\n  port: 8000\n" {
		t.Fatalf("config not restored: %q err=%v", data, err)
	}
}
