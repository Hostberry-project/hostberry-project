package main

import (
	"testing"

	"hostberry/internal/config"
)

func TestTemplatesLoad(t *testing.T) {
	config.AppConfig = &config.Config{
		Server: config.ServerConfig{Debug: false},
	}

	engine := createTemplateEngine()
	if engine == nil {
		t.Fatal("engine de templates es nil")
	}

	if err := engine.Load(); err != nil {
		t.Fatalf("error cargando/parsing templates: %v", err)
	}
}

