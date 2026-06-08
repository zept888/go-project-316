.PHONY: build test run

BINARY := bin/hexlet-go-crawler

build:
	go build -o $(BINARY) ./cmd/hexlet-go-crawler

test:
	go test ./...

run:
ifndef URL
	@echo "URL is required. Usage: make run URL=https://example.com"
	@go run ./cmd/hexlet-go-crawler --help || true
else
	-go run ./cmd/hexlet-go-crawler $(URL)
endif
