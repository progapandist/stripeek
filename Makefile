.PHONY: build run fmt vet lint check tidy clean help

BIN := stripeek
PKG := ./cmd/stripeek

help:
	@echo "Targets:"
	@echo "  build      - compile binary to ./$(BIN)"
	@echo "  run        - run proxy + TUI (listens on localhost:4242)"
	@echo "  fmt        - gofmt -w ."
	@echo "  vet        - go vet ./..."
	@echo "  lint       - staticcheck ./..."
	@echo "  check      - fmt + vet + lint + build"
	@echo "  tidy       - go mod tidy"
	@echo "  clean      - remove built binary"

build:
	go build -o $(BIN) $(PKG)

run:
	go run $(PKG)

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	@command -v staticcheck >/dev/null || { echo "install: go install honnef.co/go/tools/cmd/staticcheck@latest"; exit 1; }
	staticcheck ./...

check: fmt vet lint build

tidy:
	go mod tidy

clean:
	rm -f $(BIN)
