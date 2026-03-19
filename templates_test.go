package main

import (
	"testing"

	"hostberry/internal/config"
	webtemplates "hostberry/internal/templates"
)

func TestTemplatesLoad(t *testing.T) {
	config.AppConfig = &config.Config{
		Server: config.ServerConfig{Debug: false},
	}

	engine := webtemplates.CreateTemplateEngine(templatesFS)
	if engine == nil {
		t.Fatal("engine de templates es nil")
	}

	if err := engine.Load(); err != nil {
		t.Fatalf("error cargando/parsing templates: %v", err)
	}
}

