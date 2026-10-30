BIN := bin/jsonvalidator
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)

.PHONY: build test cover lint clean

build: ## Compila o binário em bin/
	go build -ldflags "-X github.com/marceloaugst/go-json-validator/internal/cli.Version=$(VERSION)" -o $(BIN) ./cmd/jsonvalidator

test: ## Roda os testes com detector de corrida
	go test -race ./...

cover: ## Mostra a cobertura de testes
	go test -cover ./...

lint: ## Formatação e análise estática
	gofmt -l .
	go vet ./...

clean:
	rm -rf bin
