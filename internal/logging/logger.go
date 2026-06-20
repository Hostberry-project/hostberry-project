package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config configuración de logging a fichero.
type Config struct {
	Level      string
	File       string
	MaxSizeMB  int
	MaxBackups int
}

var (
	mu       sync.Mutex
	file     *os.File
	curSize  int64
	maxBytes int64
	logPath  string
	backups  int
)

// Init configura el logger estándar de Go con salida a stdout y fichero rotativo.
func Init(cfg Config) error {
	level := strings.ToLower(strings.TrimSpace(cfg.Level))
	if level == "" {
		level = "info"
	}
	_ = level // reservado para filtrado futuro

	path := strings.TrimSpace(cfg.File)
	if path == "" {
		return nil
	}

	maxMB := cfg.MaxSizeMB
	if maxMB <= 0 {
		maxMB = 10
	}
	maxB := cfg.MaxBackups
	if maxB <= 0 {
		maxB = 5
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("crear directorio de logs: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("abrir log: %w", err)
	}

	st, _ := f.Stat()
	mu.Lock()
	if file != nil {
		_ = file.Close()
	}
	file = f
	curSize = 0
	if st != nil {
		curSize = st.Size()
	}
	logPath = path
	maxBytes = int64(maxMB) * 1024 * 1024
	backups = maxB
	mu.Unlock()

	log.SetOutput(io.MultiWriter(os.Stdout, &rotatingWriter{}))
	log.SetFlags(log.Ldate | log.Ltime)
	return nil
}

type rotatingWriter struct{}

func (rotatingWriter) Write(p []byte) (int, error) {
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return os.Stdout.Write(p)
	}
	n, err := file.Write(p)
	if err != nil {
		return n, err
	}
	curSize += int64(n)
	if maxBytes > 0 && curSize >= maxBytes {
		rotateLocked()
	}
	return n, nil
}

func rotateLocked() {
	if file == nil || logPath == "" {
		return
	}
	_ = file.Close()
	file = nil

	for i := backups - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", logPath, i)
		new := fmt.Sprintf("%s.%d", logPath, i+1)
		if i == backups-1 {
			_ = os.Remove(new)
		}
		_ = os.Rename(old, new)
	}
	_ = os.Rename(logPath, logPath+".1")

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		file = nil
		curSize = 0
		return
	}
	file = f
	curSize = 0
}
