.PHONY: build run fmt vet lint check tidy clean snapshot release-check help

BIN := stripeek
PKG := ./cmd/stripeek

help:
	@echo "Targets:"
	@echo "  build          - compile binary to ./$(BIN)"
	@echo "  run            - run proxy + TUI (listens on localhost:4242)"
	@echo "  fmt            - gofmt -w ."
	@echo "  vet            - go vet ./..."
	@echo "  lint           - staticcheck ./..."
	@echo "  check          - fmt + vet + lint + build"
	@echo "  tidy           - go mod tidy"
	@echo "  clean          - remove built binary and dist/"
	@echo "  snapshot       - build all platforms locally into ./dist/ (requires goreleaser)"
	@echo "  release-check  - validate .goreleaser.yaml without building"

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
	rm -rf dist/

snapshot:
	@command -v goreleaser >/dev/null || { echo "install: brew install goreleaser/tap/goreleaser"; exit 1; }
	goreleaser release --snapshot --clean

release-check:
	@command -v goreleaser >/dev/null || { echo "install: brew install goreleaser/tap/goreleaser"; exit 1; }
	goreleaser check
