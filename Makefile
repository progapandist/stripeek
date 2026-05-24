.PHONY: build run fmt vet lint rubocop rubocop-fix check tidy clean test-ruby help

BIN := stripeek
PKG := ./cmd/stripeek

help:
	@echo "Targets:"
	@echo "  build      - compile binary to ./$(BIN)"
	@echo "  run        - run proxy + TUI (listens on localhost:4242)"
	@echo "  fmt        - gofmt -w ."
	@echo "  vet        - go vet ./..."
	@echo "  lint       - staticcheck ./..."
	@echo "  rubocop    - lint Ruby files"
	@echo "  rubocop-fix - autocorrect Ruby files"
	@echo "  check      - fmt + vet + lint + rubocop + build"
	@echo "  tidy       - go mod tidy"
	@echo "  clean      - remove built binary"
	@echo "  test-ruby  - fire sample Stripe API calls through the proxy"

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

rubocop:
	@command -v rubocop >/dev/null || { echo "install: gem install rubocop"; exit 1; }
	rubocop scripts/

rubocop-fix:
	rubocop -A scripts/

check: fmt vet lint rubocop build

tidy:
	go mod tidy

clean:
	rm -f $(BIN)

test-ruby:
	ruby scripts/test_client.rb
