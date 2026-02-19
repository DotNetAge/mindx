# MindX Makefile - 统一构建、安装、运行入口

# 配置
BIN_DIR=bin
CMD_DIR=cmd
DASHBOARD_DIR=dashboard
SCRIPTS_DIR=scripts
DIST_DIR=dist
VERSION=$(shell cat VERSION 2>/dev/null || echo "dev")

# 颜色输出
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m

# 默认目标
.PHONY: all
all: help

# ============================================
# 核心目标 - 用户最常用的几个命令
# ============================================

.PHONY: build
build:
	@echo "$(BLUE)Building MindX...$(NC)"
	@bash $(SCRIPTS_DIR)/build.sh
	@echo "$(GREEN)✓ Build complete!$(NC)"

.PHONY: install
install: build
	@echo "$(BLUE)Installing MindX...$(NC)"
	@bash $(SCRIPTS_DIR)/install.sh
	@echo "$(GREEN)✓ Installation complete!$(NC)"

.PHONY: uninstall
uninstall:
	@echo "$(BLUE)Uninstalling MindX...$(NC)"
	@bash $(SCRIPTS_DIR)/uninstall.sh
	@echo "$(GREEN)✓ Uninstallation complete!$(NC)"

.PHONY: run
run: run-dashboard

.PHONY: dev
dev:
	@echo "$(BLUE)Starting development mode...$(NC)"
	@bash $(SCRIPTS_DIR)/dev-start.sh

.PHONY: clean
clean:
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	@rm -rf $(BIN_DIR)
	@rm -rf $(DIST_DIR)
	@rm -rf $(DASHBOARD_DIR)/dist
	@rm -rf $(DASHBOARD_DIR)/node_modules/.vite
	@rm -rf .dev
	@rm -rf .test
	@echo "$(GREEN)✓ Clean complete!$(NC)"

.PHONY: test
test:
	@echo "$(BLUE)Running tests...$(NC)"
	@echo "$(BLUE)Test workspace: $(PWD)/.test$(NC)"
	@mkdir -p $(PWD)/.test
	@MINDX_WORKSPACE=$(PWD)/.test go test ./...
	@echo "$(GREEN)✓ Tests complete!$(NC)"

.PHONY: doctor
doctor:
	@echo "$(BLUE)Running environment check...$(NC)"
	@bash $(SCRIPTS_DIR)/doctor.sh
	@echo "$(GREEN)✓ Check complete!$(NC)"

.PHONY: update
update:
	@echo "$(BLUE)Updating MindX...$(NC)"
	@echo "$(YELLOW)Step 1: Pulling latest code...$(NC)"
	@git pull
	@echo "$(GREEN)✓ Code updated$(NC)"
	@echo ""
	@echo "$(YELLOW)Step 2: Building...$(NC)"
	@make build
	@echo "$(GREEN)✓ Build complete$(NC)"
	@echo ""
	@echo "$(YELLOW)Step 3: Reinstalling...$(NC)"
	@echo "$(BLUE)Note: Your workspace files will be preserved$(NC)"
	@bash $(SCRIPTS_DIR)/install.sh
	@echo "$(GREEN)✓ Update complete!$(NC)"

# ============================================
# 构建相关
# ============================================

.PHONY: build-frontend
build-frontend:
	@echo "$(BLUE)Building dashboard...$(NC)"
	@cd $(DASHBOARD_DIR) && npm install --silent
	@cd $(DASHBOARD_DIR) && npm run build --silent
	@echo "$(GREEN)✓ Dashboard built!$(NC)"

.PHONY: build-backend
build-backend:
	@echo "$(BLUE)Building backend...$(NC)"
	@mkdir -p $(BIN_DIR)
	@if [ "$(shell uname -s)" = "Darwin" ]; then \
		echo "$(CYAN)Detected macOS, building Universal Binary...$(NC)"; \
		echo "$(YELLOW)  Building AMD64...$(NC)"; \
		CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/mindx-amd64 $(CMD_DIR)/main.go; \
		echo "$(YELLOW)  Building ARM64...$(NC)"; \
		CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/mindx-arm64 $(CMD_DIR)/main.go; \
		echo "$(YELLOW)  Creating Universal Binary...$(NC)"; \
		lipo -create -output $(BIN_DIR)/mindx $(BIN_DIR)/mindx-amd64 $(BIN_DIR)/mindx-arm64; \
		rm -f $(BIN_DIR)/mindx-amd64 $(BIN_DIR)/mindx-arm64; \
		echo "$(GREEN)✓ Universal Binary built!$(NC)"; \
		lipo -info $(BIN_DIR)/mindx; \
	else \
		CGO_ENABLED=1 go build -o $(BIN_DIR)/mindx $(CMD_DIR)/main.go; \
		echo "$(GREEN)✓ Backend built!$(NC)"; \
	fi

.PHONY: build-all
build-all:
	@echo "$(BLUE)Building for all platforms...$(NC)"
	@mkdir -p $(BIN_DIR)
	@echo "$(YELLOW)Building Linux AMD64...$(NC)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/mindx-linux-amd64 $(CMD_DIR)/main.go
	@echo "$(GREEN)✓ mindx-linux-amd64$(NC)"
	@echo "$(YELLOW)Building Linux ARM64...$(NC)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BIN_DIR)/mindx-linux-arm64 $(CMD_DIR)/main.go
	@echo "$(GREEN)✓ mindx-linux-arm64$(NC)"
	@echo "$(YELLOW)Building macOS AMD64...$(NC)"
	@CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/mindx-darwin-amd64 $(CMD_DIR)/main.go
	@echo "$(GREEN)✓ mindx-darwin-amd64$(NC)"
	@echo "$(YELLOW)Building macOS ARM64...$(NC)"
	@CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/mindx-darwin-arm64 $(CMD_DIR)/main.go
	@echo "$(GREEN)✓ mindx-darwin-arm64$(NC)"
	@echo "$(YELLOW)Building Windows AMD64...$(NC)"
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/mindx-windows-amd64.exe $(CMD_DIR)/main.go
	@echo "$(GREEN)✓ mindx-windows-amd64.exe$(NC)"
	@echo "$(GREEN)✓ Cross-platform builds complete!$(NC)"

.PHONY: build-linux-release
build-linux-release:
	@echo "$(BLUE)Building Linux release packages...$(NC)"
	@bash $(SCRIPTS_DIR)/build-linux.sh

.PHONY: build-windows-release
build-windows-release:
	@echo "$(BLUE)Building Windows release packages...$(NC)"
	@bash $(SCRIPTS_DIR)/build-windows.sh

.PHONY: build-all-releases
build-all-releases:
	@echo "$(BLUE)Building all release packages...$(NC)"
	@bash $(SCRIPTS_DIR)/build-linux.sh
	@bash $(SCRIPTS_DIR)/build-windows.sh

# ============================================
# 运行相关
# ============================================

.PHONY: run-dashboard
run-dashboard:
	@echo "$(BLUE)Starting Dashboard...$(NC)"
	@if [ -f $(BIN_DIR)/mindx ]; then \
		$(BIN_DIR)/mindx dashboard; \
	else \
		go run $(CMD_DIR)/main.go dashboard; \
	fi

.PHONY: run-tui
run-tui:
	@echo "$(BLUE)Starting TUI...$(NC)"
	@if [ -f $(BIN_DIR)/mindx ]; then \
		$(BIN_DIR)/mindx tui; \
	else \
		go run $(CMD_DIR)/main.go tui; \
	fi

.PHONY: run-kernel
run-kernel:
	@echo "$(BLUE)Starting Kernel service...$(NC)"
	@if [ -f $(BIN_DIR)/mindx ]; then \
		$(BIN_DIR)/mindx kernel run; \
	else \
		go run $(CMD_DIR)/main.go kernel run; \
	fi

.PHONY: run-train
run-train:
	@echo "$(BLUE)Running training...$(NC)"
	@if [ -f $(BIN_DIR)/mindx ]; then \
		$(BIN_DIR)/mindx train --run-once; \
	else \
		go run $(CMD_DIR)/main.go train --run-once; \
	fi

.PHONY: run-model-test
run-model-test:
	@echo "$(BLUE)Testing model...$(NC)"
	@if [ -f $(BIN_DIR)/mindx ]; then \
		$(BIN_DIR)/mindx model test; \
	else \
		go run $(CMD_DIR)/main.go model test; \
	fi

.PHONY: run-skill-list
run-skill-list:
	@echo "$(BLUE)Listing skills...$(NC)"
	@if [ -f $(BIN_DIR)/mindx ]; then \
		$(BIN_DIR)/mindx skill list; \
	else \
		go run $(CMD_DIR)/main.go skill list; \
	fi

# ============================================
# 开发辅助
# ============================================

.PHONY: fmt
fmt:
	@echo "$(BLUE)Formatting code...$(NC)"
	@go fmt ./...
	@cd $(DASHBOARD_DIR) && npm run fmt 2>/dev/null || true
	@echo "$(GREEN)✓ Format complete!$(NC)"

.PHONY: lint
lint:
	@echo "$(BLUE)Linting code...$(NC)"
	@go vet ./...
	@cd $(DASHBOARD_DIR) && npm run lint 2>/dev/null || true
	@echo "$(GREEN)✓ Lint complete!$(NC)"

.PHONY: deps
deps:
	@echo "$(BLUE)Updating dependencies...$(NC)"
	@go mod tidy
	@go mod download
	@cd $(DASHBOARD_DIR) && npm update 2>/dev/null || true
	@echo "$(GREEN)✓ Dependencies updated!$(NC)"

.PHONY: version
version:
	@echo "$(BLUE)MindX Version: $(VERSION)$(NC)"
	@echo "Go Version: $(shell go version)"
	@cd $(DASHBOARD_DIR) && node -v 2>/dev/null || true
	@cd $(DASHBOARD_DIR) && npm -v 2>/dev/null || true

.PHONY: help
help:
	@echo "$(GREEN)╔═══════════════════════════════════════════════════════════════╗$(NC)"
	@echo "$(GREEN)║               MindX Makefile                           ║$(NC)"
	@echo "$(GREEN)╚═══════════════════════════════════════════════════════════════╝$(NC)"
	@echo ""
	@echo "$(YELLOW)【核心命令 - 快速开始】$(NC)"
	@echo "  $(BLUE)make build$(NC)        - Build MindX (frontend + backend)"
	@echo "  $(BLUE)make install$(NC)      - Install MindX to system"
	@echo "  $(BLUE)make update$(NC)       - Update to latest version"
	@echo "  $(BLUE)make uninstall$(NC)    - Uninstall MindX from system"
	@echo "  $(BLUE)make run$(NC)          - Start Dashboard"
	@echo "  $(BLUE)make dev$(NC)          - Start development mode"
	@echo "  $(BLUE)make clean$(NC)        - Clean build artifacts"
	@echo "  $(BLUE)make test$(NC)         - Run tests"
	@echo "  $(BLUE)make doctor$(NC)       - Check environment for issues"
	@echo ""
	@echo "$(YELLOW)【构建命令】$(NC)"
	@echo "  $(BLUE)make build-frontend$(NC)  - Build frontend only"
	@echo "  $(BLUE)make build-backend$(NC)   - Build backend only"
	@echo "  $(BLUE)make build-all$(NC)       - Build for all platforms (binaries only)"
	@echo "  $(BLUE)make build-linux-release$(NC) - Build full Linux release packages (AMD64 + ARM64)"
	@echo "  $(BLUE)make build-windows-release$(NC) - Build full Windows release packages (AMD64)"
	@echo "  $(BLUE)make build-all-releases$(NC) - Build ALL release packages (Linux + Windows)"
	@echo ""
	@echo "$(YELLOW)【运行命令】$(NC)"
	@echo "  $(BLUE)make run-dashboard$(NC)   - Start Dashboard"
	@echo "  $(BLUE)make run-tui$(NC)        - Start TUI chat"
	@echo "  $(BLUE)make run-kernel$(NC)     - Start Kernel service"
	@echo "  $(BLUE)make run-train$(NC)      - Run training once"
	@echo "  $(BLUE)make run-model-test$(NC) - Test model compatibility"
	@echo "  $(BLUE)make run-skill-list$(NC)- List all skills"
	@echo ""
	@echo "$(YELLOW)【开发辅助】$(NC)"
	@echo "  $(BLUE)make fmt$(NC)           - Format code"
	@echo "  $(BLUE)make lint$(NC)          - Lint code"
	@echo "  $(BLUE)make deps$(NC)          - Update dependencies"
	@echo "  $(BLUE)make version$(NC)        - Show version info"
	@echo "  $(BLUE)make help$(NC)           - Show this help"
	@echo ""
	@echo "$(YELLOW)【示例】$(NC)"
	@echo "  # 完整流程: make build && make install"
	@echo "  # 开发:     make dev"
	@echo "  # 运行:     make run"
	@echo "  # Linux发布: make build-linux-release"
	@echo ""
