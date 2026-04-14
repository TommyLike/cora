BINARY     := bin/cora
CMD        := ./cmd/cora
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)
ARGS       := services list

# ── Build ─────────────────────────────────────────────────────────────────────

.PHONY: build
build:
	@mkdir -p $(dir $(BINARY))
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

.PHONY: build-prod
build-prod:
	@mkdir -p $(dir $(BINARY))
	CGO_ENABLED=0 go build -ldflags "-s -w $(LDFLAGS)" -o $(BINARY) $(CMD)

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" $(CMD)

.PHONY: clean
clean:
	rm -rf bin/

# ── Test ──────────────────────────────────────────────────────────────────────

.PHONY: test
test:
	go test -race ./...

.PHONY: test-unit
test-unit:
	go test -race -short ./...

.PHONY: test-cover
test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: test-cover-text
test-cover-text:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# ── Lint & Format ─────────────────────────────────────────────────────────────

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	gofmt -w .
	goimports -w . 2>/dev/null || true

.PHONY: vet
vet:
	go vet ./...

# ── Dependencies ──────────────────────────────────────────────────────────────

.PHONY: deps
deps:
	go mod download

.PHONY: tidy
tidy:
	go mod tidy

# ── Docker ────────────────────────────────────────────────────────────────────

IMAGE ?= cora

.PHONY: docker-build
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

.PHONY: docker-run
docker-run:
	docker run --rm \
		-v ./config.example.yaml:/root/.config/cora/config.yaml:ro \
		$(IMAGE):latest $(ARGS)

# ── Help ──────────────────────────────────────────────────────────────────────

.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Build:"
	@echo "  build          Build binary ($(BINARY))"
	@echo "  build-prod     Build optimised binary (CGO disabled, stripped)"
	@echo "  install        Install to GOPATH/bin"
	@echo "  clean          Remove build artefacts"
	@echo ""
	@echo "Test:"
	@echo "  test           Run all tests with race detector"
	@echo "  test-unit      Run short tests only (no integration)"
	@echo "  test-cover     Generate HTML coverage report"
	@echo "  test-cover-text Print coverage summary to stdout"
	@echo ""
	@echo "Quality:"
	@echo "  lint           Run golangci-lint"
	@echo "  fmt            Format code (gofmt + goimports)"
	@echo "  vet            Run go vet"
	@echo ""
	@echo "Dependencies:"
	@echo "  deps           Download modules"
	@echo "  tidy           Run go mod tidy"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build   Build Docker image (IMAGE=$(IMAGE) VERSION=$(VERSION))"
	@echo "  docker-run     Run CLI inside Docker (pass ARGS='forum posts list')"
