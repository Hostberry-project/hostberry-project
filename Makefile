.PHONY: build run clean test install deps dev

# Variables
BINARY_NAME=hostberry
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_RUN=$(GO_CMD) run
GO_TEST=$(GO_CMD) test
GO_MOD=$(GO_CMD) mod

# Build para sistema actual
build: deps
	@echo "🔨 Compilando $(BINARY_NAME)..."
	$(GO_BUILD) -ldflags="-s -w" -o $(BINARY_NAME) .
	@echo "✅ Compilado: $(BINARY_NAME)"

# Build para Raspberry Pi 3 (ARM)
build-arm: deps
	@echo "🔨 Compilando para Raspberry Pi 3 (ARM)..."
	CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 $(GO_BUILD) -ldflags="-s -w" -o $(BINARY_NAME)-arm .
	@echo "✅ Compilado: $(BINARY_NAME)-arm"
	@echo "📦 Copia $(BINARY_NAME)-arm a tu Raspberry Pi y ejecuta: ./$(BINARY_NAME)-arm"

# Ejecutar en modo desarrollo
run:
	@echo "🚀 Ejecutando $(BINARY_NAME)..."
	$(GO_RUN) main.go

# Instalar dependencias
deps:
	@echo "📦 Instalando dependencias..."
	@if ! command -v $(GO_CMD) > /dev/null; then \
		echo "❌ Error: Go no está instalado"; \
		echo "   Instala Go con: sudo apt install golang-go"; \
		echo "   O descarga desde: https://go.dev/dl/"; \
		exit 1; \
	fi
	$(GO_MOD) download
	$(GO_MOD) tidy
	@echo "✅ Dependencias instaladas"

# Limpiar archivos compilados
clean:
	@echo "🧹 Limpiando..."
	rm -f $(BINARY_NAME) $(BINARY_NAME)-arm
	$(GO_CMD) clean

# Ejecutar tests
test:
	@echo "🧪 Ejecutando tests..."
	$(GO_TEST) -v ./...

# Modo desarrollo con hot-reload (requiere air)
dev:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "⚠️  Air no instalado. Instalando..."; \
		go install github.com/cosmtrek/air@latest; \
		air; \
	fi

# Instalar herramientas de desarrollo
install-tools:
	@echo "🛠️  Instalando herramientas..."
	$(GO_CMD) install github.com/cosmtrek/air@latest
	$(GO_CMD) install golang.org/x/tools/cmd/goimports@latest

# Formatear código
fmt:
	@echo "📝 Formateando código..."
	$(GO_CMD) fmt ./...

# Verificar código
vet:
	@echo "🔍 Verificando código..."
	$(GO_CMD) vet ./...

# Linter (requiere golangci-lint)
lint:
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint no instalado. Instalando..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2; \
		golangci-lint run; \
	fi
