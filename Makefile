# Kanban CLI Makefile

BINARY_NAME=kanban
BIN_DIR=bin
GO_IMAGE=golang:1.23-alpine
DOCKER_RUN=docker run --rm -v $(CURDIR):/app -w /app $(GO_IMAGE)

PREFIX ?= /usr/local
SYSTEMD_DIR ?= /etc/systemd/system

.PHONY: all build test clean install uninstall install-timer uninstall-timer install-completion uninstall-completion purge purge-xdg purge-legacy docker-build docker-run init mod-init pkg-arch pkg-debian pkg-all

# Default target
all: build

# Initialize go module (run once)
mod-init:
	$(DOCKER_RUN) go mod init github.com/kiracore/kanban
	$(DOCKER_RUN) go mod tidy

# Download dependencies
deps:
	$(DOCKER_RUN) go mod download

# Build binary using Docker (pure Go, no CGO needed)
build:
	@mkdir -p $(BIN_DIR)
	$(DOCKER_RUN) sh -c "CGO_ENABLED=0 go build -ldflags='-s -w' -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/kanban"

# Build for multiple platforms
build-all:
	@mkdir -p $(BIN_DIR)
	$(DOCKER_RUN) sh -c "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/kanban"
	$(DOCKER_RUN) sh -c "CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags='-s -w' -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/kanban"
	$(DOCKER_RUN) sh -c "CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags='-s -w' -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/kanban"
	$(DOCKER_RUN) sh -c "CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags='-s -w' -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/kanban"

# Run tests (no CGO needed with pure Go sqlite)
test:
	$(DOCKER_RUN) go test -v ./...

# Run tests with coverage
test-coverage:
	$(DOCKER_RUN) sh -c "go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out"

# Run only config tests (no CGO needed)
test-config:
	$(DOCKER_RUN) go test -v ./internal/config/...

# Format code
fmt:
	$(DOCKER_RUN) go fmt ./...

# Lint
lint:
	$(DOCKER_RUN) sh -c "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && golangci-lint run"

# Clean build artifacts
clean:
	rm -rf $(BIN_DIR)

# Install binary
install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(BIN_DIR)/$(BINARY_NAME) $(DESTDIR)$(PREFIX)/bin/

# Uninstall binary
uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/$(BINARY_NAME)

# Install systemd timer (run as root)
install-timer:
	install -m 755 scripts/kanban-sync.sh $(DESTDIR)$(PREFIX)/bin/
	install -m 644 scripts/kanban-sync.service $(DESTDIR)$(SYSTEMD_DIR)/kanban-sync@.service
	install -m 644 scripts/kanban-sync.timer $(DESTDIR)$(SYSTEMD_DIR)/kanban-sync@.timer
	systemctl daemon-reload
	@echo "Enable with: systemctl enable --now kanban-sync@\$$USER.timer"

# Uninstall systemd timer
uninstall-timer:
	-systemctl disable kanban-sync@*.timer 2>/dev/null || true
	rm -f $(DESTDIR)$(SYSTEMD_DIR)/kanban-sync@.service
	rm -f $(DESTDIR)$(SYSTEMD_DIR)/kanban-sync@.timer
	rm -f $(DESTDIR)$(PREFIX)/bin/kanban-sync.sh
	systemctl daemon-reload

# Install everything
install-all: install install-timer install-completion

# Uninstall everything (keeps user data)
uninstall-all: uninstall-timer uninstall uninstall-completion

# Install shell completion (bash)
install-completion: build
	@mkdir -p /etc/bash_completion.d
	$(BIN_DIR)/$(BINARY_NAME) completion bash > /etc/bash_completion.d/$(BINARY_NAME)
	@echo "Bash completion installed. Run 'source /etc/bash_completion.d/$(BINARY_NAME)' or restart shell."

# Uninstall shell completion
uninstall-completion:
	rm -f /etc/bash_completion.d/$(BINARY_NAME)

# Purge user data (XDG paths) - USE WITH CAUTION
purge: purge-xdg

purge-xdg:
	@echo "This will remove XDG data:"
	@echo "  - Config: ~/.config/kanban/"
	@echo "  - Data:   ~/.local/share/kanban/"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	rm -rf $(HOME)/.config/kanban
	rm -rf $(HOME)/.local/share/kanban

# Purge legacy user data (pre-XDG paths)
purge-legacy:
	@echo "This will remove legacy data:"
	@echo "  - Config: ~/.kanban.yaml"
	@echo "  - Database: ~/.kanban/"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	rm -f $(HOME)/.kanban.yaml
	rm -rf $(HOME)/.kanban

# Build Docker image
docker-build:
	docker build -t kanban:latest .

# Run in Docker with gh auth
docker-run:
	docker run --rm -it \
		-v $(HOME)/.config/gh:/root/.config/gh:ro \
		-v $(PWD):/workspace \
		-w /workspace \
		kanban:latest $(ARGS)

# Development shell
dev-shell:
	$(DOCKER_RUN) sh

# Add dependencies
add-deps:
	$(DOCKER_RUN) go get github.com/spf13/cobra@latest
	$(DOCKER_RUN) go get github.com/spf13/viper@latest
	$(DOCKER_RUN) go get gopkg.in/yaml.v3@latest
	$(DOCKER_RUN) go mod tidy

# Sync labels to all repos in config
sync-labels:
	./$(BIN_DIR)/$(BINARY_NAME) sync --labels-only

# Deploy workflow to all repos
deploy-workflow:
	./scripts/deploy-workflow.sh

deploy-workflow-dry:
	./scripts/deploy-workflow.sh --dry-run

# Package building
pkg-arch:
	@echo "Building AUR package..."
	./packaging/arch/build.sh

pkg-debian:
	@echo "Building Debian package..."
	./packaging/debian/build.sh

pkg-all: pkg-arch pkg-debian

# Help
help:
	@echo "Kanban CLI - Build Targets"
	@echo ""
	@echo "Build:"
	@echo "  build         - Build binary"
	@echo "  build-all     - Build for all platforms"
	@echo "  test          - Run tests"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  clean         - Remove build artifacts"
	@echo ""
	@echo "Install:"
	@echo "  install            - Install binary to $(PREFIX)/bin"
	@echo "  uninstall          - Remove binary"
	@echo "  install-timer      - Install systemd timer (root)"
	@echo "  uninstall-timer    - Remove systemd timer (root)"
	@echo "  install-completion - Install bash completion"
	@echo "  uninstall-completion - Remove bash completion"
	@echo "  install-all        - Install binary + timer + completion"
	@echo "  uninstall-all      - Remove all installed files"
	@echo "  purge              - Remove user data (XDG paths)"
	@echo "  purge-xdg          - Remove XDG data (~/.config/kanban, ~/.local/share/kanban)"
	@echo "  purge-legacy       - Remove legacy data (~/.kanban.yaml, ~/.kanban/)"
	@echo ""
	@echo "Packaging:"
	@echo "  pkg-arch      - Build AUR package via Docker"
	@echo "  pkg-debian    - Build Debian package via Docker"
	@echo "  pkg-all       - Build all packages"
	@echo ""
	@echo "Deploy:"
	@echo "  sync-labels       - Sync labels to all repos"
	@echo "  deploy-workflow   - Deploy triage workflow to all repos"
	@echo "  deploy-workflow-dry - Dry run workflow deploy"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run in Docker (ARGS='...')"
	@echo "  dev-shell     - Start dev shell"
	@echo ""
	@echo "Setup:"
	@echo "  mod-init      - Initialize go module"
	@echo "  deps          - Download dependencies"
	@echo "  add-deps      - Add Go dependencies"
