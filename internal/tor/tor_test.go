package tor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTorInstalledKnownPath(t *testing.T) {
	dir := t.TempDir()
	fakeTor := filepath.Join(dir, "tor")
	if err := os.WriteFile(fakeTor, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	prev := torBinaryPaths
	t.Cleanup(func() { torBinaryPaths = prev })
	torBinaryPaths = []string{fakeTor}
	if !isTorInstalled() {
		t.Fatal("expected tor installed via known path")
	}
}

func TestIsTorInstalledMissing(t *testing.T) {
	prev := torBinaryPaths
	t.Cleanup(func() { torBinaryPaths = prev })
	torBinaryPaths = []string{filepath.Join(t.TempDir(), "missing-tor")}
	if isTorInstalled() {
		t.Fatal("expected tor not installed")
	}
}
