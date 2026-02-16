VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Colors
C_RESET  := \033[0m
C_BOLD   := \033[1m
C_DIM    := \033[2m
C_GREEN  := \033[32m
C_YELLOW := \033[33m
C_CYAN   := \033[36m
C_WHITE  := \033[37m

.DEFAULT_GOAL := help

.PHONY: build install test lint clean all install-hooks hooks help i

## Build ──────────────────────────────────────────────

all: build ## Build everything (alias for build)

build: ## Build mem binary for current OS/arch
	go build $(LDFLAGS) -o bin/mem ./cmd/mem

build-all: build-linux build-darwin build-windows ## Cross-compile for all platforms

build-linux: ## Build for Linux (amd64 + arm64)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/mem-linux-amd64 ./cmd/mem
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/mem-linux-arm64 ./cmd/mem

build-darwin: ## Build for macOS (amd64 + arm64)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/mem-darwin-amd64 ./cmd/mem
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/mem-darwin-arm64 ./cmd/mem

build-windows: ## Build for Windows (amd64)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/mem-windows-amd64.exe ./cmd/mem

install: ## Install mem into $GOPATH/bin
	go install $(LDFLAGS) ./cmd/mem

## Test ───────────────────────────────────────────────

test: ## Run all tests
	go test -v ./...

test-cover: ## Run tests with coverage report (HTML)
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## Code Quality ───────────────────────────────────────

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format Go source files
	go fmt ./...

tidy: ## Tidy go.mod / go.sum
	go mod tidy

## Housekeeping ───────────────────────────────────────

clean: ## Remove build artifacts and coverage files
	rm -rf bin/
	rm -f coverage.out coverage.html

install-hooks: ## Install lefthook git hooks
	go install github.com/evilmartians/lefthook@latest
	lefthook install

hooks: install-hooks ## Alias for install-hooks

## Meta ───────────────────────────────────────────────

i: ## Interactive target picker (fzf/peco/gum)
	@target=$$($(MAKE) -s _list-targets | \
		if command -v fzf >/dev/null 2>&1; then \
			fzf --ansi --reverse --header="Pick a make target" --preview="$(MAKE) -s _desc TARGET={}"; \
		elif command -v peco >/dev/null 2>&1; then \
			peco --prompt="make> "; \
		elif command -v gum >/dev/null 2>&1; then \
			gum filter --placeholder="Pick a make target"; \
		else \
			echo "No fuzzy finder found. Install fzf, peco, or gum." >&2; exit 1; \
		fi) && \
	if [ -n "$$target" ]; then \
		printf "\n$(C_CYAN)▶ make %s$(C_RESET)\n\n" "$$target"; \
		$(MAKE) $$target; \
	fi

help: ## Show this help
	@printf "\n$(C_BOLD)$(C_CYAN)  memories$(C_RESET)$(C_DIM) — make targets$(C_RESET)\n"
	@printf "$(C_DIM)  ─────────────────────────────────────────$(C_RESET)\n\n"
	@awk '\
		/^## / { \
			gsub(/^## /, ""); \
			section = $$0; \
			next \
		} \
		/^[a-zA-Z_-]+:.*##/ { \
			target = $$1; \
			gsub(/:.*/, "", target); \
			desc = $$0; \
			sub(/.*## */, "", desc); \
			if (section != prev) { \
				printf "  $(C_DIM)%s$(C_RESET)\n", section; \
				prev = section \
			} \
			printf "  $(C_GREEN)%-18s$(C_RESET) %s\n", target, desc \
		}' $(MAKEFILE_LIST)
	@printf "\n$(C_DIM)  Tip: run $(C_RESET)$(C_YELLOW)make i$(C_RESET)$(C_DIM) to pick a target interactively$(C_RESET)\n\n"

# Internal helpers (not shown in help)
_list-targets:
	@awk '/^[a-zA-Z_-]+:.*##/ { target=$$1; gsub(/:.*/, "", target); print target }' $(MAKEFILE_LIST) | grep -v '^_'

_desc:
	@awk -v t="$(TARGET):" '$$1 == t && /##/ { sub(/.*## */, ""); print }' $(MAKEFILE_LIST)
