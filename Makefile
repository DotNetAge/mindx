# =============================================================================
# MindX Makefile - Comprehensive Build System
# =============================================================================
# MindX AI Agent CLI - Build, Test, Run, Install, Deploy, and Release
#
# Usage:
#   make help              # 显示所有可用目标（默认）
#   make build             # 编译二进制文件
#   make install           # 编译并安装到系统路径
#   make run               # 运行 TUI（默认模式）
#   make run-daemon        # 运行 Daemon 服务
#   make restart           # 编译并重启 Daemon（通过 launchd/systemd，非阻塞）
#   make test              # 运行所有测试
#   make clean             # 清理构建产物
#   make docs              # 生成文档
#   make release           # 发布多平台版本
#   make docker-build      # 构建 Docker 镜像
#
# =============================================================================

.PHONY: build build-all setup-cross clear install run run-daemon restart stop test clean docs help \
        dev uninstall \
        release release-notes release-publish release-homebrew release-winget publish \
        docker .env docker-build docker-run docker-run-daemon docker-push docker-release docker-clean \
        ci cd deps-update \
        pre-commit version info changelog \
        build-debug dev-watch test-verbose test-specific \
        vulnerability-check deps-graph \
        docs-serve readme clean-all setup-hooks clear-creds \
        lint fmt

# =============================================================================
# 配置变量
# =============================================================================

# 项目信息
BINARY_NAME    ?= mindx
DOCKER_USER    ?= dotnetage
VERSION        ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v2.1.0")
BUILD_TIME     ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY      ?= $(shell git diff --quiet HEAD 2>/dev/null && echo "clean" || echo "dirty")

# Go 工具链
GO             ?= go

# Go 版本信息
GOVERSION      ?= $(shell $(GO) version | grep -oP 'go\d+\.\d+' | head -1)

# 版本号（不含 v 前缀）
VERSION_NUM    := $(VERSION:v%=%)

# 构建标志
LDFLAGS        ?= -s -w \
                 -X github.com/DotNetAge/mindx/internal/core.Version=$(VERSION_NUM) \
                 -X github.com/DotNetAge/mindx/internal/core.Commit=$(GIT_COMMIT) \
                 -X github.com/DotNetAge/mindx/internal/core.BuildTime=$(BUILD_TIME) \
                 -X github.com/DotNetAge/mindx/internal/core.Dirty=$(GIT_DIRTY)

GOFLAGS        ?= -trimpath -ldflags "$(LDFLAGS)"

# 目录配置
BUILD_DIR      ?= ./dist
COVERAGE_DIR   ?= ./coverage
DOCS_DIR       ?= ./docs/api

# 发布配置
GITHUB_REPO    ?= DotNetAge/mindx
HOMEBREW_TAP   ?= DotNetAge/homebrew-tap

# 颜色输出（增强可读性）
RED            := \033[0;31m
GREEN          := \033[0;32m
YELLOW         := \033[0;33m
BLUE           := \033[0;34m
PURPLE         := \033[0;35m
CYAN           := \033[0;36m
NC             := \033[0m
BOLD           := \033[1m

# =============================================================================
# 代码质量门禁（所有 build 前自动执行）
# =============================================================================

## fmt: 检查并自动修复 Go 代码格式（gofmt）
fmt:
	@echo "$(CYAN)📐 Checking Go formatting (gofmt)...$(NC)"
	@UNFORMATTED=$$(find . -name '*.go' -not -path './vendor/*' -not -path './.git/*' | xargs gofmt -l 2>/dev/null); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "$(YELLOW)⚠  Formatting issues found, auto-fixing:$(NC)"; \
		echo "$$UNFORMATTED" | while read f; do echo "  fixing $$f"; done; \
		find . -name '*.go' -not -path './vendor/*' -not -path './.git/*' | xargs gofmt -w; \
		echo "$(GREEN)✅ All files formatted.$(NC)"; \
	else \
		echo "$(GREEN)✅ All .go files properly formatted.$(NC)"; \
	fi

## lint: 运行 golangci-lint 代码检查
lint:
	@echo "$(CYAN)🔍 Running golangci-lint...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
		echo "$(GREEN)✅ Lint passed.$(NC)"; \
	else \
		echo "$(YELLOW)⚠  golangci-lint not found, installing...$(NC)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run ./...; \
		echo "$(GREEN)✅ Lint passed.$(NC)"; \
	fi

# =============================================================================
# 主要构建目标
# =============================================================================

## build: 编译当前平台二进制至 dist/ (前置: fmt → pre-build)
build: fmt pre-build
	@echo "$(GREEN)➡ Building $(BINARY_NAME) v$(VERSION_NUM) for $(shell $(GO) env GOOS)/$(shell $(GO) env GOARCH)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "$(GREEN)✅ Build complete!$(NC)"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)

## build-all: 编译所有主流平台二进制至 dist/ (前置: fmt → lint)
build-all: fmt lint pre-build build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64
	@echo "$(GREEN)✅ All platforms built! Check $(BUILD_DIR)/$(NC)"

## setup-cross: 安装交叉编译工具链（用于 Linux/Windows 目标）
setup-cross:
	@echo "$(GREEN)➡ Installing cross-compilation toolchains...$(NC)"
	@if command -v brew >/dev/null 2>&1; then \
		brew install FiloSottile/musl-cross/musl-cross mingw-w64; \
	else \
		echo "$(YELLOW)⚠  Homebrew not found. Install manually:$(NC)"; \
		echo "  Linux:   brew install FiloSottile/musl-cross/musl-cross"; \
		echo "  Windows: brew install mingw-w64"; \
	fi
	@echo "$(GREEN)✅ Cross-compilation toolchains installed! Run 'make build-all' for all platforms.$(NC)"

## build-debug: 编译调试版本（带符号信息，前置: fmt → lint）
build-debug: fmt lint pre-build
	@echo "$(YELLOW)🔧 Building debug binary...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug .
	@echo "$(GREEN)✅ Debug build complete: $(BUILD_DIR)/$(BINARY_NAME)-debug$(NC)"

## install: 构建并部署到 ~/.mindx（含 runtime 资源 + PATH + 系统服务配置，前置: fmt → lint）
install: fmt lint
	@echo "$(GREEN)➡ Copying runtime files...$(NC)"
	@cp -r runtime/* ~/.mindx/ && \
		echo "$(GREEN)  ✅ runtime/ → ~/.mindx/$(NC)"
	@echo "$(GREEN)➡ Building $(BINARY_NAME) → ~/.mindx/bin/$(BINARY_NAME)...$(NC)"
	@mkdir -p ~/.mindx/bin ~/.mindx/settings
	@rm -f ~/.mindx/bin/$(BINARY_NAME)
	@CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o ~/.mindx/bin/$(BINARY_NAME) . && \
		echo "$(GREEN)  ✅ $(BINARY_NAME) → ~/.mindx/bin/$(BINARY_NAME)$(NC)"
	@# ── PATH 配置 ──
	@SHELL_RC=""; \
	if [ "$$SHELL" = "/bin/zsh" ] || [ "$$SHELL" = "/usr/bin/zsh" ]; then \
		SHELL_RC="$$HOME/.zshrc"; \
	elif [ "$$SHELL" = "/bin/bash" ] || [ "$$SHELL" = "/usr/bin/bash" ]; then \
		SHELL_RC="$$HOME/.bashrc"; \
	else \
		SHELL_RC="$$HOME/.profile"; \
	fi; \
	LINE='export PATH="$$HOME/.mindx/bin:$$PATH"'; \
	if ! grep -qxF "$$LINE" "$$SHELL_RC" 2>/dev/null; then \
		echo "" >> "$$SHELL_RC"; \
		echo "# MindX" >> "$$SHELL_RC"; \
		echo "$$LINE" >> "$$SHELL_RC"; \
		echo "$(GREEN)  ✅ PATH added to $$SHELL_RC$(NC)"; \
	else \
		echo "$(GREEN)  ✅ PATH already in $$SHELL_RC$(NC)"; \
	fi
	@# ── 系统服务配置 + 注册 + 重启 ──
	@MINDX_BIN="$$HOME/.mindx/bin/$(BINARY_NAME)"; \
	HOME_DIR=$$HOME; \
	UNAME_S=$$(uname -s); \
	if [ "$$UNAME_S" = "Darwin" ]; then \
		PLIST="$$HOME/.mindx/settings/com.mindx.daemon.plist"; \
		LABEL="com.mindx.daemon"; \
		LAUNCH_AGENTS="$$HOME/Library/LaunchAgents"; \
		mkdir -p "$$LAUNCH_AGENTS" "$$HOME/.mindx/logs"; \
		echo "$(CYAN)  ⟳ Cleaning up old services...$(NC)" && \
		for old_plist in "$$LAUNCH_AGENTS"/com.mindx.*.plist "$$LAUNCH_AGENTS"/com.dotnetage.mindx.plist; do \
			if [ -f "$$old_plist" ]; then \
				old_label=$$(basename "$$old_plist" .plist); \
				launchctl unload "$$old_plist" 2>/dev/null && echo "     unloaded $$old_label" || true; \
				rm -f "$$old_plist" && echo "     removed $$old_plist"; \
			fi; \
		done; \
		printf '%s\n' \
			'<?xml version="1.0" encoding="UTF-8"?>' \
			'<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
			'<plist version="1.0">' \
			'<dict>' \
			'    <key>Label</key>' \
			"    <string>$$LABEL</string>" \
			'    <key>ProgramArguments</key>' \
			'    <array>' \
			"        <string>$$MINDX_BIN</string>" \
			'        <string>daemon</string>' \
			'    </array>' \
			'    <key>RunAtLoad</key>' \
			'    <true/>' \
			'    <key>KeepAlive</key>' \
			'    <true/>' \
			'    <key>StandardOutPath</key>' \
			"    <string>$$HOME_DIR/.mindx/logs/daemon.log</string>" \
			'    <key>StandardErrorPath</key>' \
			"    <string>$$HOME_DIR/.mindx/logs/daemon.err</string>" \
			'    <key>EnvironmentVariables</key>' \
			'    <dict>' \
			'        <key>HOME</key>' \
			"        <string>$$HOME_DIR</string>" \
			'    </dict>' \
			'    <key>ProcessType</key>' \
			'    <string>Interactive</string>' \
			'</dict>' \
			'</plist>' \
			> "$$PLIST"; \
		echo "$(GREEN)  ✅ launchd plist → $$PLIST$(NC)"; \
		cp "$$PLIST" "$$LAUNCH_AGENTS/$$LABEL.plist"; \
		SERVICE="gui/$$(id -u)/$$LABEL"; \
		echo "$(CYAN)  ⟳ Stopping existing service...$(NC)" && \
		(launchctl bootout "$$SERVICE" 2>/dev/null && echo "     stopped" || echo "     (not running)"); \
		echo "$(CYAN)  ⟳ Starting service (bootstrap)...$(NC)" && \
		launchctl bootstrap gui/$$(id -u) "$$LAUNCH_AGENTS/$$LABEL.plist" && \
		echo "$(GREEN)  ✅ Daemon registered and started$(NC)" || \
		echo "$(RED)  ❌ Bootstrap failed, trying legacy load...$(NC)" && \
		launchctl load "$$LAUNCH_AGENTS/$$LABEL.plist" 2>/dev/null; \
	fi; \
	if [ "$$UNAME_S" = "Linux" ]; then \
		SERVICE_PATH="$$HOME/.mindx/settings/$(BINARY_NAME).service"; \
		SERVICE_NAME="$(BINARY_NAME)"; \
		mkdir -p "$$HOME/.mindx/logs"; \
		printf '%s\n' \
			'[Unit]' \
			'Description=MindX AI Agent Daemon' \
			'After=network.target' \
			'' \
			'[Service]' \
			'Type=simple' \
			"ExecStart=$$MINDX_BIN start" \
			'Restart=on-failure' \
			'RestartSec=5' \
			'' \
			'[Install]' \
			'WantedBy=default.target' \
			> "$$SERVICE_PATH"; \
		echo "$(GREEN)  ✅ systemd unit → $$SERVICE_PATH$(NC)"; \
		mkdir -p "$$HOME/.config/systemd/user"; \
		cp "$$SERVICE_PATH" "$$HOME/.config/systemd/user/$$SERVICE_NAME.service"; \
		echo "$(CYAN)  ⟳ Stopping existing service...$(NC)" && \
		systemctl --user stop "$$SERVICE_NAME" 2>/dev/null && echo "     stopped" || echo "     (not running)"; \
		echo "$(CYAN)  ⟳ Enabling and starting service...$(NC)" && \
		systemctl --user enable "$$SERVICE_NAME" && \
		systemctl --user start "$$SERVICE_NAME" && \
		echo "$(GREEN)  ✅ Daemon registered and started$(NC)"; \
	fi
	@echo ""
	@echo "$(GREEN)🎉 Installation complete!$(NC)"
	@echo ""
	@echo "  Run: exec $$SHELL   (or source your rc file)"
	@echo "  Then: $(BINARY_NAME)"

## uninstall: 卸载 mindx（停止服务 + 清理二进制/配置/PATH）
uninstall:
	@echo "$(RED)🗑️ Uninstalling $(BINARY_NAME)...$(NC)"
	@echo ""
	@# ── 停止正在运行的 daemon ──
	@echo "$(CYAN)  ⟳ Stopping daemon...$(NC)"
	@UNAME_S=$$(uname -s); \
	if [ "$$UNAME_S" = "Darwin" ]; then \
		SERVICE="gui/$$(id -u)/com.mindx.daemon"; \
		(launchctl bootout "$$SERVICE" 2>/dev/null && \
			echo "     stopped launchd service" || echo "     (no launchd service)"); \
	elif [ "$$UNAME_S" = "Linux" ]; then \
		systemctl --user stop $(BINARY_NAME) 2>/dev/null && \
			echo "     stopped systemd service" || echo "     (no systemd service)"; \
		systemctl --user disable $(BINARY_NAME) 2>/dev/null || true; \
	fi; \
	pkill -f "$(BINARY_NAME) daemon" 2>/dev/null && echo "     killed mindx daemon" || true; \
	lsof -ti:1313 -ti:1314 2>/dev/null | xargs kill 2>/dev/null && echo "     freed ports 1313/1314" || true
	@sleep 1
	@echo ""
	@# ── 删除服务文件 ──
	@echo "$(CYAN)  ⟳ Removing service files...$(NC)"
	@UNAME_S=$$(uname -s); \
	if [ "$$UNAME_S" = "Darwin" ]; then \
		rm -f ~/Library/LaunchAgents/com.mindx.daemon.plist && \
			echo "     removed launchd plist" || true; \
		rm -f ~/Library/LaunchAgents/com.dotnetage.mindx.plist 2>/dev/null || true; \
	elif [ "$$UNAME_S" = "Linux" ]; then \
		rm -f ~/.config/systemd/user/$(BINARY_NAME).service && \
			systemctl --user daemon-reload 2>/dev/null && \
			echo "     removed systemd unit" || true; \
	fi
	@rm -f ~/.mindx/settings/com.mindx.daemon.plist 2>/dev/null || true
	@rm -f ~/.mindx/settings/com.dotnetage.mindx.plist 2>/dev/null || true
	@rm -f ~/.mindx/settings/$(BINARY_NAME).service 2>/dev/null || true
	@echo ""
	@# ── 从 shell rc 中移除 PATH ──
	@echo "$(CYAN)  ⟳ Removing PATH from shell rc files...$(NC)"
	@for rc in "$$HOME/.zshrc" "$$HOME/.bashrc" "$$HOME/.bash_profile" "$$HOME/.profile"; do \
		if [ -f "$$rc" ]; then \
			grep -v '^# MindX$$' "$$rc" | grep -v '^export PATH=".*\.mindx/bin' > "$$rc.tmp" && \
				mv "$$rc.tmp" "$$rc" && echo "     cleaned $$(basename $$rc)" || true; \
		fi; \
	done
	@echo ""
	@# ── 删除安装文件 ──
	@echo "$(CYAN)  ⟳ Removing installed files...$(NC)"
	@rm -rf ~/.mindx/bin && echo "     removed ~/.mindx/bin" || true
	@rm -rf ~/.mindx/settings && echo "     removed ~/.mindx/settings" || true
	@rm -rf ~/.mindx/logs && echo "     removed ~/.mindx/logs" || true
	@rm -f /usr/local/bin/$(BINARY_NAME) 2>/dev/null || true
	@rm -f $$GOPATH/bin/$(BINARY_NAME) 2>/dev/null || true
	@echo ""
	@# ── Windows 清理（仅在 Windows 上生效）──
	@schtasks /delete /tn MindXDaemon /f 2>/dev/null && echo "     removed Windows scheduled task" || true
	@powershell -NoProfile -NonInteractive -Command \
		"$$d=[Environment]::GetFolderPath('Desktop'); \
		 Remove-Item \"$$d/mindx.lnk\" -Force -ErrorAction SilentlyContinue" 2>/dev/null || true
	@echo ""
	@echo "$(GREEN)✅ Uninstall complete!$(NC)"
	@echo "   ~/.mindx/agents, memory, sessions, and config were kept."
	@echo "   To remove them too, run: rm -rf ~/.mindx"

# =============================================================================
# 运行目标
# =============================================================================

## run: 编译并启动 TUI（默认模式）
run: build
	@echo "$(YELLOW)🚀 Starting TUI (Terminal UI)...$(NC)"
	@echo "$(CYAN)💡 Tips:$(NC)"
	@echo "  • Enter messages and press Enter to send"
	@echo "  • Type /help for available commands"
	@echo "  • Press Ctrl+C to exit"
	@echo ""
	./$(BUILD_DIR)/$(BINARY_NAME)

## run-daemon: 编译并启动 Daemon 服务
run-daemon: build
	@echo "$(YELLOW)🔧 Starting Daemon service...$(NC)"
	@echo "$(CYAN)💡 Service info:$(NC)"
	@echo "  • WebSocket: ws://localhost:1314/ws"
	@echo "  • Press Ctrl+C to stop"
	@echo ""
	./$(BUILD_DIR)/$(BINARY_NAME) daemon

## restart: 编译并重启 daemon（通过系统服务管理器，非阻塞）
restart: build
	@echo "$(YELLOW)🔄 Restarting mindx daemon...$(NC)"
	@echo "$(CYAN)➡ Building → ~/.mindx/bin/$(BINARY_NAME)...$(NC)"
	@mkdir -p ~/.mindx/bin
	@cp -f $(BUILD_DIR)/$(BINARY_NAME) ~/.mindx/bin/$(BINARY_NAME) && \
		echo "$(GREEN)  ✅ Binary updated.$(NC)"
	@echo "$(CYAN)➡ Deploying frontend → ~/.mindx/web/...$(NC)"
	@if [ -d "../mindx-chat/dist" ]; then \
		rm -rf $$HOME/.mindx/web 2>/dev/null; \
		cp -r ../mindx-chat/dist $$HOME/.mindx/web && \
			echo "$(GREEN)  ✅ Frontend deployed.$(NC)" || \
			echo "$(YELLOW)  ⚠ Frontend not found at ../mindx-chat/dist (skip).$(NC)"; \
	else \
		echo "$(YELLOW)  ⚠ ../mindx-chat/dist not found, skipping frontend deploy.$(NC)"; \
	fi
	@UNAME_S=$$(uname -s); \
	if [ "$$UNAME_S" = "Darwin" ]; then \
		LABEL="com.mindx.daemon"; \
		SERVICE="gui/$$(id -u)/$$LABEL"; \
		PLIST="$$HOME/Library/LaunchAgents/$$LABEL.plist"; \
		if launchctl print "$$SERVICE" >/dev/null 2>&1; then \
			echo "$(CYAN)  ⟳ Restarting via launchctl kickstart...$(NC)" && \
			launchctl kickstart -k "$$SERVICE" && \
			echo "$(GREEN)✅ Daemon restarted via launchd.$(NC)" || \
			(echo "$(YELLOW)  ⚠ kickstart failed, starting directly...$(NC)" && \
			launchctl bootout "$$SERVICE" 2>/dev/null; \
			$(BUILD_DIR)/$(BINARY_NAME) daemon & \
			echo "$(GREEN)✅ Daemon started in background.$(NC)"); \
		else \
			echo "$(CYAN)  ⟳ Service not loaded, bootstrapping...$(NC)" && \
			launchctl bootstrap gui/$$(id -u) "$$PLIST" && \
			echo "$(GREEN)✅ Daemon started via launchd.$(NC)" || \
			echo "$(YELLOW)⚠ Bootstrap failed, starting directly...$(NC)" && \
			$(BUILD_DIR)/$(BINARY_NAME) daemon & \
			echo "$(GREEN)✅ Daemon started in background.$(NC)"; \
		fi; \
	elif [ "$$UNAME_S" = "Linux" ]; then \
		systemctl --user restart $(BINARY_NAME) 2>/dev/null && \
			echo "$(GREEN)✅ Daemon restarted via systemd.$(NC)" || \
			echo "$(RED)❌ Failed to restart via systemd. Try: make run-daemon$(NC)"; \
	fi

## stop: 停止本机 mindx daemon
stop:
	@echo "$(YELLOW)🛑 Stopping mindx daemon...$(NC)"
	@UNAME_S=$$(uname -s); \
	if [ "$$UNAME_S" = "Darwin" ]; then \
		SERVICE="gui/$$(id -u)/com.mindx.daemon"; \
		launchctl bootout "$$SERVICE" 2>/dev/null && echo "     stopped launchd service" || true; \
	fi; \
	pkill -f "$(BINARY_NAME) daemon" 2>/dev/null || \
	 (lsof -ti:1313 -ti:1314 2>/dev/null | xargs kill 2>/dev/null) || \
	 echo "$(GREEN)  ✅ No running daemon found.$(NC)"
	@sleep 1
	@echo "$(GREEN)✅ mindx daemon stopped.$(NC)"

## dev: 开发模式（go run，不编译，推荐日常开发）
dev:
	@echo "$(YELLOW)🛠️  Running in development mode...$(NC)"
	$(GO) run .

## dev-watch: 文件监控自动重载（需要 air）
dev-watch:
	@command -v air >/dev/null 2>&1 && \
		(air -c .air.toml) || \
		(echo "$(YELLOW)⚠ air not installed. Install with: go install github.com/cosmtrek/air@latest$(NC)" && exit 1)

# =============================================================================
# 测试目标
# =============================================================================

## test: 运行所有单元测试（带覆盖率报告）
test:
	@echo "$(GREEN)▶ Running tests with coverage...$(NC)"
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -race -coverprofile=$(COVERAGE_DIR)/coverage.out -v ./...
	@echo ""
	@echo "$(GREEN)✅ Tests complete! Coverage summary:$(NC)"
	@$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1

## test-verbose: 详细测试输出（包含所有包）
test-verbose:
	@echo "$(GREEN)▶ Running all tests with verbose output...$(NC)"
	$(GO) test -v ./...

## test-specific: 运行特定测试函数
# 用法: make test-specific TESTFUNC=TestDefaultApp
test-specific:
	@echo "$(GREEN)▶ Running specific test: $(TESTFUNC)...$(NC)"
	$(GO) test -run $(TESTFUNC) -v ./...

# =============================================================================
# 文档目标
# =============================================================================

## docs: 生成 API 文档
docs:
	@echo "$(GREEN)📝 Generating documentation...$(NC)"
	@mkdir -p $(DOCS_DIR)
	@if command -v godoc >/dev/null 2>&1; then \
		godoc -html -output $(DOCS_DIR) ./internal/core/ 2>/dev/null || echo "$(YELLOW)⚠ Some packages may not have docs$(NC)"; \
	else \
		echo "$(YELLOW)⚠ godoc not available. Installing pkgsite instead...$(NC)"; \
		go install golang.org/x/pkgsite/cmd/pkgsite@latest; \
	fi
	@echo "$(GREEN)✅ Documentation generated in $(DOCS_DIR)/$(NC)"

## docs-serve: 启动本地文档服务器
docs-serve:
	@echo "$(BLUE)🌐 Starting documentation server...$(NC)"
	@echo "$(CYAN)Open http://localhost:6060 in your browser$(NC)"
	godoc -http=:6060 2>/dev/null || pkgsite

## readme: 生成 README 的 TOC 和 badge 更新
readme:
	@echo "$(GREEN)📝 Updating README...$(NC)"
	@if command -v markdown-toc >/dev/null 2>&1; then \
		markdown-toc -i README.md ; \
	else \
		echo "$(YELLOW)⚠ markdown-toc not installed.$(NC)"; \
	fi
	@echo "$(GREEN)✅ README updated!$(NC)"

## changelog: 从 git 历史生成 CHANGELOG.md
changelog:
	@echo "$(GREEN)📝 Generating changelog...$(NC)"
	@if command -v git-chglog >/dev/null 2>&1; then \
		git-chglog -o CHANGELOG.md ; \
	else \
		git log --pretty=format:"- %s (%h)" --since="1 month ago" > CHANGELOG.md.tmp ; \
		mv CHANGELOG.md.tmp CHANGELOG.md ; \
	fi
	@echo "$(GREEN)✅ Changelog generated!$(NC)"

## build-linux-amd64: Linux x86_64
build-linux-amd64:
	@echo "$(GREEN)➡ Building for linux/amd64...$(NC)"
	@mkdir -p $(BUILD_DIR)/linux-amd64
	@if command -v x86_64-linux-musl-gcc >/dev/null 2>&1; then \
		CGO_ENABLED=1 CC=x86_64-linux-musl-gcc GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) . && \
		echo "$(GREEN)✅ linux/amd64 done$(NC)"; \
	else \
		echo "$(YELLOW)⚠  linux/amd64 skipped — install: brew install FiloSottile/musl-cross/musl-cross$(NC)"; \
	fi

## build-linux-arm64: Linux ARM64
build-linux-arm64:
	@echo "$(GREEN)➡ Building for linux/arm64...$(NC)"
	@mkdir -p $(BUILD_DIR)/linux-arm64
	@if command -v aarch64-linux-musl-gcc >/dev/null 2>&1; then \
		CGO_ENABLED=1 CC=aarch64-linux-musl-gcc GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/linux-arm64/$(BINARY_NAME) . && \
		echo "$(GREEN)✅ linux/arm64 done$(NC)"; \
	else \
		echo "$(YELLOW)⚠  linux/arm64 skipped — install: brew install FiloSottile/musl-cross/musl-cross$(NC)"; \
	fi

## build-darwin-amd64: macOS Intel
build-darwin-amd64:
	@echo "$(GREEN)➡ Building for darwin/amd64...$(NC)"
	@mkdir -p $(BUILD_DIR)/darwin-amd64
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/darwin-amd64/$(BINARY_NAME) .
	@echo "$(GREEN)✅ darwin/amd64 done$(NC)"

## build-darwin-arm64: macOS Apple Silicon (M1/M2)
build-darwin-arm64:
	@echo "$(GREEN)➡ Building for darwin/arm64 (Apple Silicon)...$(NC)"
	@mkdir -p $(BUILD_DIR)/darwin-arm64
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/darwin-arm64/$(BINARY_NAME) .
	@echo "$(GREEN)✅ darwin/arm64 done$(NC)"

## build-windows-amd64: Windows x86_64
build-windows-amd64:
	@echo "$(GREEN)➡ Building for windows/amd64...$(NC)"
	@mkdir -p $(BUILD_DIR)/windows-amd64
	@if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		if CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/windows-amd64/$(BINARY_NAME).exe . 2>/dev/null; then \
			echo "$(GREEN)✅ windows/amd64 done (CGO)$(NC)"; \
		else \
			echo "$(YELLOW)⚠  windows/amd64 CGO build failed, trying CGO_ENABLED=0...$(NC)"; \
			CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/windows-amd64/$(BINARY_NAME).exe . && \
			echo "$(GREEN)✅ windows/amd64 done (CGO_ENABLED=0)$(NC)" || \
			echo "$(RED)❌ windows/amd64 failed$(NC)"; \
		fi \
	else \
		echo "$(YELLOW)⚠  windows/amd64 skipped — install: brew install mingw-w64$(NC)"; \
	fi

# =============================================================================
# 发布目标
# =============================================================================

REL = releases
V   = $(VERSION_NUM)

# 辅助：打包一个平台为 tar.gz（或 .zip），跳过缺失的二进制
define package-platform
	@mkdir -p $(REL)
	@platform_dir="$(BUILD_DIR)/$(1)-$(2)"; \
	binary="$$platform_dir/$(BINARY_NAME)$(3)"; \
	if [ ! -f "$$binary" ]; then \
		echo "  ⚠  $(1)-$(2) not found, skipping"; \
		exit 0; \
	fi; \
	archive="$(REL)/$(BINARY_NAME)-$(V)-$(1)-$(2).tar.gz"; \
	if [ "$(1)" = "windows" ]; then \
		archive="$(REL)/$(BINARY_NAME)-$(V)-$(1)-$(2).zip"; \
		(cd "$$platform_dir" && zip -q "$$OLDPWD/$$archive" "$(BINARY_NAME).exe"); \
	else \
		tar czf "$$archive" -C "$$platform_dir" "$(BINARY_NAME)"; \
	fi; \
	echo "  ✅ $$(basename $$archive)  $$(ls -lh $$archive | awk '{print $$5}')"
endef

## release: 交叉编译并打包所有平台（带 checksums）
release: clean build-all
	@echo "$(GREEN)📦 Packaging...$(NC)"
	$(call package-platform,darwin,amd64,)
	$(call package-platform,darwin,arm64,)
	$(call package-platform,linux,amd64,)
	$(call package-platform,linux,arm64,)
	$(call package-platform,windows,amd64,.exe)
	@echo "$(GREEN)🔐 Checksums...$(NC)"
	@cd $(REL) && shasum -a 256 *.tar.gz 2>/dev/null > checksums.txt; shasum -a 256 *.zip 2>/dev/null >> checksums.txt; cat checksums.txt
	@echo "$(GREEN)✅ Release packages in $(REL)/$(NC)"
	@ls -lh $(REL)/

## release-notes: 生成发布说明（写入 RELEASE_NOTES.md）
release-notes:
	@echo "$(GREEN)📝 Generating release notes...$(NC)"
	@printf '%s\n' "# $(BINARY_NAME) $(V) Release Notes" "" > RELEASE_NOTES.md
	@printf '%s\n' "## 📦 Downloads" "" >> RELEASE_NOTES.md
	@printf '| %s | %s | %s |\n' "Platform" "File" "SHA256" >> RELEASE_NOTES.md
	@printf '|%s|%s|%s|\n' "----------" "------" "------" >> RELEASE_NOTES.md
	@{ \
		for f in $(REL)/*.tar.gz $(REL)/*.zip; do \
			[ -f "$$f" ] || continue; \
			platform=$$(basename "$$f" | sed 's/\.tar\.gz$$//;s/\.zip$$//;s/^[^-]*-[^-]*-//'); \
			sha=$$(shasum -a 256 "$$f" | cut -d' ' -f1); \
			printf '| %s | %s | %s |\n' "$$platform" "$$(basename "$$f")" "$$sha"; \
		done; \
	} >> RELEASE_NOTES.md
	@printf '%s\n' "" "## ✨ Changes" "" >> RELEASE_NOTES.md
	@git log --oneline --no-merges "$$(git describe --tags --abbrev=0 2>/dev/null)..HEAD" 2>/dev/null >> RELEASE_NOTES.md || echo "- Initial release" >> RELEASE_NOTES.md
	@echo "$(GREEN)✅ RELEASE_NOTES.md created$(NC)"

## release-publish: 编译 → 打包 → GitHub Release
release-publish: release
	@echo "$(GREEN)🚀 Creating GitHub Release v$(V)...$(NC)"
	@if gh release view "v$(V)" --repo "$(GITHUB_REPO)" &>/dev/null; then \
		echo "  ⚠  Release v$(V) exists, uploading assets..."; \
		gh release upload "v$(V)" --repo "$(GITHUB_REPO)" --clobber $(REL)/*; \
	else \
		gh release create "v$(V)" \
			--repo "$(GITHUB_REPO)" \
			--title "$(BINARY_NAME) v$(V)" \
			--notes "Release v$(V)" \
			$(REL)/*; \
	fi
	@echo "$(GREEN)✅ GitHub Release v$(V) published$(NC)"

## release-homebrew: 从 GitHub Release 下载 darwin 包 → 生成 Homebrew formula → 推送 tap
## 前置条件: release 已发布到 GitHub (make release-publish 或 CI 自动完成)
release-homebrew:
	@echo "$(GREEN)🍺 Generating Homebrew formula from GitHub Release...$(NC)"
	@bash scripts/homebrew-release.sh
	@echo "$(GREEN)✅ Formula generated in dist/$(NC)"

## release-winget: 提交 winget-pkgs PR（需 Windows zip 已发布到 GitHub Release）
release-winget:
	@echo "$(GREEN)📦 Submitting winget-pkgs PR...$(NC)"
	@SHA256=$$(shasum -a 256 "$(REL)/$(BINARY_NAME)-$(V)-windows-amd64.zip" 2>/dev/null | cut -d' ' -f1); \
	if [ -z "$$SHA256" ]; then \
		echo "$(RED)❌ Windows zip not found in $(REL)/$(NC)"; \
		exit 1; \
	fi; \
	WINGET_DIR=/tmp/winget-pkgs; \
	MANIFEST_DIR="manifests/d/DotNetAge/Mindx/$(V)"; \
	MANIFEST_PATH="$$MANIFEST_DIR/DotNetAge.Mindx.yaml"; \
	GH_REPO="DotNetAge/mindx"; \
	GIT_USER=$$(git config user.name); \
	GIT_EMAIL=$$(git config user.email); \
	rm -rf "$$WINGET_DIR"; \
	gh repo fork microsoft/winget-pkgs --clone --remote=false 2>/dev/null || true; \
	git clone --depth=1 "https://github.com/$$GIT_USER/winget-pkgs.git" "$$WINGET_DIR" 2>/dev/null || { \
		echo "$(YELLOW)⚠  Fork not found, cloning upstream...$(NC)"; \
		git clone --depth=1 "https://github.com/microsoft/winget-pkgs.git" "$$WINGET_DIR"; \
		cd "$$WINGET_DIR" && gh repo fork --remote=false; \
		cd "$(CURDIR)"; \
	}; \
	mkdir -p "$$WINGET_DIR/$$MANIFEST_DIR"; \
	cat > "$$WINGET_DIR/$$MANIFEST_PATH" <<-MANIFEST
	PackageIdentifier: DotNetAge.Mindx
	PackageVersion: $(V)
	PackageLocale: en-US
	Publisher: DotNetAge
	PublisherUrl: https://github.com/$(GH_REPO)
	PackageName: MindX
	License: MIT
	ShortDescription: MindX - AI-native multi-agent conversation platform
	Tags: AI agent cli llm mindx
	Installers:
	  - Architecture: x64
	    InstallerType: zip
	    NestedInstallerType: portable
	    NestedInstallerFiles:
	      - RelativeFilePath: $(BINARY_NAME).exe
	        PortableCommandAlias: mindx
	    InstallerUrl: https://github.com/$(GH_REPO)/releases/download/v$(V)/$(BINARY_NAME)-$(V)-windows-amd64.zip
	    InstallerSha256: $$SHA256
	ManifestType: singleton
	ManifestVersion: 1.9.0
	MANIFEST; \
	cd "$$WINGET_DIR" && \
	git add "$$MANIFEST_PATH" && \
	git -c user.name="$$GIT_USER" -c user.email="$$GIT_EMAIL" \
		commit -m "DotNetAge.Mindx v$(V)" && \
	git push origin HEAD:main 2>&1 && \
	gh pr create \
		--repo microsoft/winget-pkgs \
		--head "$$GIT_USER:main" \
		--title "DotNetAge.Mindx version $(V)" \
		--body "New version: **$(V)**\n\n- Package: DotNetAge.Mindx\n- URL: https://github.com/$(GH_REPO)" \
		--label "package-submission" && \
	echo "$(GREEN)✅ winget-pkgs PR submitted!$(NC)" || \
	echo "$(YELLOW)⚠  PR creation failed. Manifest saved at $$WINGET_DIR/$$MANIFEST_PATH$(NC)"; \
	cd "$(CURDIR)"

## publish: 一键发布 — 打标签 → 编译 → GitHub Release → Homebrew → Winget
publish:
	@echo "$(GREEN)═══════════════════════════════════════════════════════════════$(NC)"
	@echo "$(GREEN)  MindX 一键发布管道$(NC)"
	@echo "$(GREEN)═══════════════════════════════════════════════════════════════$(NC)"
	@echo ""
	@# ── 前置检查 ──
	@current_branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$current_branch" != "main" ]; then \
		echo "$(RED)❌ 必须在 main 分支上发布 (当前: $$current_branch)$(NC)"; \
		echo "  git checkout main && git pull"; \
		exit 1; \
	fi
	@if ! git diff --quiet HEAD; then \
		echo "$(RED)❌ 工作区有未提交的变更，请先提交$(NC)"; \
		git status --short; \
		exit 1; \
	fi
	@if ! command -v gh >/dev/null 2>&1; then \
		echo "$(RED)❌ 需要 gh CLI: brew install gh$(NC)"; \
		exit 1; \
	fi
	@if ! gh auth status 2>/dev/null; then \
		echo "$(RED)❌ gh 未登录: gh auth login$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✅ 前置检查通过$(NC)"
	@echo ""
	@# ── 版本管理 ──
	@current_tag="$$(git describe --tags --abbrev=0 2>/dev/null || echo 'v0.0.0')"; \
	current_ver="$${current_tag#v}"; \
	major=$$(echo "$$current_ver" | cut -d. -f1); \
	minor=$$(echo "$$current_ver" | cut -d. -f2); \
	patch=$$(echo "$$current_ver" | cut -d. -f3); \
	new_patch=$$((patch + 1)); \
	new_ver="$${major}.$${minor}.$${new_patch}"; \
	new_tag="v$${new_ver}"; \
	echo "  当前版本:  $$current_tag"; \
	echo "  发布版本:  $$new_tag"; \
	echo ""; \
	read -p "  确认发布 $$new_tag ? [Enter/N]: " confirm; \
	if [ "$$confirm" != "" ] && [ "$$confirm" != "y" ] && [ "$$confirm" != "Y" ]; then \
		echo "$(YELLOW)⚠ 已取消$(NC)"; \
		exit 0; \
	fi; \
	echo ""; \
	echo "$(GREEN)▸ 创建标签 $$new_tag ...$(NC)"; \
	git tag "$$new_tag" && git push origin "$$new_tag"; \
	echo "$(GREEN)✅ 标签已推送$(NC)"; \
	export V=$${new_ver} && export VERSION="$${new_tag}"
	@echo ""
	@# ── 编译 + 打包 + 校验 ──
	@$(MAKE) release
	@echo ""
	@# ── GitHub Release ──
	@$(MAKE) release-publish
	@echo ""
	@# ── Homebrew ──
	@$(MAKE) release-homebrew
	@echo ""
	@# ── Winget ──
	@read -p "  提交 winget-pkgs PR? [y/N]: " winget_confirm; \
	if [ "$$winget_confirm" = "y" ] || [ "$$winget_confirm" = "Y" ]; then \
		$(MAKE) release-winget; \
	fi
	@echo ""
	@# ── Docker (本地构建推送，需 runtime/bin/mindx 和 runtime/data) ──
	@read -p "  推送 Docker 镜像? (make docker-release) [y/N]: " docker_confirm; \
	if [ "$$docker_confirm" = "y" ] || [ "$$docker_confirm" = "Y" ]; then \
		$(MAKE) docker-release; \
	fi
	@echo ""
	@echo "$(GREEN)═══════════════════════════════════════════════════════════════$(NC)"
	@echo "$(GREEN)  🎉 发布完成!$(NC)"
	@echo "$(GREEN)═══════════════════════════════════════════════════════════════$(NC)"
	@echo ""
	@echo "  GitHub:   https://github.com/$(GITHUB_REPO)/releases/tag/v$(VERSION_NUM)"
	@echo "  Homebrew: brew install $(HOMEBREW_TAP)/$(BINARY_NAME)"
	@echo "  Winget:   winget install DotNetAge.Mindx"
	@echo "  Docker:   docker pull $(BINARY_NAME):v$(VERSION_NUM)"
	@echo ""

# =============================================================================
# Docker 目标
# =============================================================================

## docker: 从 git 动态读取版本 → 生成 .env → 编译 Linux 二进制 → Docker Compose 构建并启动
## 镜像 tag = git tag (如 v2.2.0)，无 tag 时为 dev
docker:
	@echo "$(GREEN)🐳 Building MindX Docker image...$(NC)"
	@# ── 从 git 动态提取版本信息 ──
	@_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	_BUILD_TIME=$$(date -u '+%Y-%m-%dT%H:%M:%SZ'); \
	echo "$(CYAN)  Version:   $$_TAG (from git tag)$(NC)"; \
	echo "$(CYAN)  Commit:    $$_COMMIT$(NC)"; \
	echo "$(CYAN)  BuildTime: $$_BUILD_TIME$(NC)"; \
	echo ""; \
	# ── 写入 .env（Docker Compose 自动读取）── \
	echo "# Auto-generated by 'make docker' — do not edit manually" > .env; \
	echo "MINDX_VERSION=$$_TAG" >> .env; \
	echo "MINDX_COMMIT=$$_COMMIT" >> .env; \
	echo "MINDX_BUILD_TIME=$$_BUILD_TIME" >> .env; \
	echo "$(GREEN)📝 .env generated from git → MINDX_VERSION=$$_TAG$(NC)"; \
	echo ""; \
	@# ── 清理旧二进制，防止 //go:embed 递归嵌套打包 ── \
	rm -f runtime/bin/mindx; \
	mkdir -p $(BUILD_DIR)/linux-amd64 runtime/bin; \
	if [ -n "$$CC" ] && command -v $$CC >/dev/null 2>&1; then \
		echo "$(GREEN)➡ Compiling linux/amd64 (CGO, $$CC)...$(NC)"; \
		CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) .; \
	else \
		echo "$(GREEN)➡ Compiling linux/amd64 (pure Go, no CGO)...$(NC)"; \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) .; \
	fi && \
	echo "$(GREEN)📦 Copying binary → runtime/bin/mindx...$(NC)" && \
	cp $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) runtime/bin/mindx && \
	echo "$(GREEN)🐳 Building Docker image (mindx:$$_TAG)...$(NC)" && \
	docker compose build && \
	echo "$(GREEN)✅ Build complete. Starting daemon...$(NC)" && \
	docker compose up -d && \
	echo "" && \
	echo "$(GREEN)🎉 MindX daemon is running!$(NC)" && \
	echo "   Image:    $(CYAN)mindx:$$_TAG$(NC)" && \
	echo "   Web UI:   $(CYAN)http://localhost:1313$(NC)" && \
	echo "   WebSocket: $(CYAN)ws://localhost:1314/ws$(NC)" && \
	echo "$(YELLOW)   Logs: 'docker compose logs -f'$(NC)" && \
	echo "$(YELLOW)   Stop: 'docker compose down'$(NC)"

## .env: 手动生成 .env（不构建镜像）
.env:
	@_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	_BUILD_TIME=$$(date -u '+%Y-%m-%dT%H:%M:%SZ'); \
	echo "# Auto-generated from git" > .env; \
	echo "MINDX_VERSION=$$_TAG" >> .env; \
	echo "MINDX_COMMIT=$$_COMMIT" >> .env; \
	echo "MINDX_BUILD_TIME=$$_BUILD_TIME" >> .env; \
	echo "$(GREEN)✅ .env written: MINDX_VERSION=$$_TAG COMMIT=$$_COMMIT$(NC)"

## docker-build: 构建 Docker 镜像（支持自定义版本参数）
## 用法: make docker-build VERSION=v2.2.0
docker-build:
	@echo "$(GREEN)🐳 Building Docker image...$(NC)"
	@_VER=${VERSION}; \
	docker build \
		-t $(DOCKER_USER)/$(BINARY_NAME):$${_VER} \
		-t $(DOCKER_USER)/$(BINARY_NAME):latest \
		--build-arg VERSION=$${_VER} \
		--build-arg COMMIT=${GIT_COMMIT} \
		--build-arg BUILD_TIME="${BUILD_TIME}" \
		. ; \
	echo "$(GREEN)✅ Docker image built: $(DOCKER_USER)/$(BINARY_NAME):$${_VER}$(NC)"

## docker-run: 运行 Docker 容器（TUI 模式）
docker-run:
	@echo "$(YELLOW)🐳 Running Docker container (TUI mode)...$(NC)"
	docker run -it --rm \
		-v ~/.mindx:/home/mindx/.mindx \
		--name $(BINARY_NAME)-tui \
		$(DOCKER_USER)/$(BINARY_NAME):latest

## docker-run-daemon: 运行 Docker 容器（Daemon 模式，端口映射完整）
docker-run-daemon:
	@echo "$(YELLOW)🐳 Running Docker container (Daemon mode)...$(NC)"
	docker run -d \
		--name $(BINARY_NAME)-daemon \
		-p 1313:1313 \
		-p 1314:1314 \
		-v ~/.mindx:/home/mindx/.mindx \
		--restart unless-stopped \
		$(DOCKER_USER)/$(BINARY_NAME):latest start

## docker-push: 推送 Docker 镜像到仓库
docker-push:
	@echo "$(GREEN)📤 Pushing Docker image...$(NC)"
	docker push $(DOCKER_USER)/$(BINARY_NAME):$(VERSION)
	docker push $(DOCKER_USER)/$(BINARY_NAME):latest
	@echo "$(GREEN)✅ Images pushed successfully!$(NC)"

## docker-release: 本地构建并推送 Docker 镜像（版本跟随最新 git tag，前置: fmt → lint）
## 前置条件:
##   1. 已打 tag 且已 push 到远程（git tag v2.x.x && git push origin v2.x.x）
##   2. 工作区干净（无未提交变更）
##   3. 已登录 Docker Hub（docker login）
##   4. runtime/bin/mindx 和 runtime/data/ 存在（本地构建产物 / .gitignored 文件）
##
## 流程: 校验 → 编译 linux/amd64 → 填充 runtime/bin/ → 生成 .env → docker build → 推送
docker-release: fmt lint
	@echo "$(GREEN)═══════════════════════════════════════════════════════════════$(NC)"
	@echo "$(GREEN)  🐳 MindX Docker Release (Local Build & Push)$(NC)"
	@echo "$(GREEN)═══════════════════════════════════════════════════════════════$(NC)"
	@echo ""
	@# ── 前置检查 ──
	@if ! command -v docker >/dev/null 2>&1; then \
		echo "$(RED)❌ docker 未安装$(NC)"; exit 1; \
	fi
	@if ! docker info >/dev/null 2>&1; then \
		echo "$(RED)❌ Docker 未运行或未登录，请先: docker login$(NC)"; exit 1; \
	fi
	@_TAG=$$(git describe --tags --abbrev=0 2>/dev/null); \
	if [ -z "$$_TAG" ] || [ "$$_TAG" = "dev" ]; then \
		echo "$(RED)❌ 未找到 git tag，请先打标签: git tag v2.x.x && git push origin v2.x.x$(NC)"; exit 1; \
	fi; \
	if ! git diff --quiet HEAD 2>/dev/null; then \
		echo "$(RED)❌ 工作区有未提交的变更，请先提交$(NC)"; git status --short; exit 1; \
	fi; \
	echo "$(GREEN)✅ 前置检查通过$(NC)"; \
	echo "$(CYAN)  Tag:       $$_TAG$(NC)"; \
	echo ""
	@# ── 版本信息 ──
	@_TAG=$$(git describe --tags --abbrev=0 2>/dev/null); \
	_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null); \
	_BUILD_TIME=$$(date -u '+%Y-%m-%dT%H:%M:%SZ'); \
	_VER=$${_TAG#v}; \
	echo "$(CYAN)  Version:   $$_TAG$(NC)"; \
	echo "$(CYAN)  Commit:    $$_COMMIT$(NC)"; \
	echo "$(CYAN)  BuildTime: $$_BUILD_TIME$(NC)"
	@echo ""
	@# ── 编译 linux/amd64 二进制 → runtime/bin/mindx ──
	@echo "$(GREEN)➡ Building linux/amd64 → runtime/bin/mindx ...$(NC)"
	@# ── 清理旧二进制，防止 //go:embed 递归嵌套打包 ──
	@rm -f runtime/bin/mindx
	@mkdir -p $(BUILD_DIR)/linux-amd64 runtime/bin; \
	if command -v x86_64-linux-musl-gcc >/dev/null 2>&1; then \
		echo "$(CYAN)   Using musl cross-compiler (CGO)$(NC)"; \
		CGO_ENABLED=1 CC=x86_64-linux-musl-gcc GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) .; \
	else \
		echo "$(CYAN)   Using pure Go (CGO_ENABLED=0)$(NC)"; \
		echo "$(YELLOW)   Tip: brew install FiloSottile/musl-cross/musl-cross for better compatibility$(NC)"; \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) .; \
	fi && \
	cp $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) runtime/bin/mindx && \
	chmod +x runtime/bin/mindx && \
	echo "$(GREEN)✅ Binary built → runtime/bin/mindx$(NC)"
	@echo ""
	@# ── 校验二进制版本与 git tag 一致 ──
	@echo "$(GREEN)🔍 Verifying binary version against git tag ...$(NC)"; \
	_BUILT_VER=$$(runtime/bin/mindx version 2>&1 | grep "Version:" | awk '{print $$2}'); \
	_EXPECTED_VER="$(VERSION_NUM)"; \
	echo "$(CYAN)   Built version : $$_BUILT_VER$(NC)"; \
	echo "$(CYAN)   Expected (tag): $$_EXPECTED_VER$(NC)"; \
	if [ "$$_BUILT_VER" = "$$_EXPECTED_VER" ] || [ "$$_BUILT_VER" = "$$_TAG" ]; then \
		echo "$(GREEN)✅ Version match confirmed!$(NC)"; \
	else \
		echo "$(RED)❌ Version mismatch! Binary reports '$$_BUILT_VER' but git tag is '$$_TAG' ($$_EXPECTED_VER)$(NC)"; \
		exit 1; \
	fi
	@echo ""
	@# ── 检查 runtime/data ──
	@if [ ! -d runtime/data ] || [ -z "$$(ls -A runtime/data 2>/dev/null)" ]; then \
		echo "$(YELLOW)⚠  runtime/data/ 为空或不存在（模型文件缺失），镜像将不包含本地模型$(NC)"; \
	fi
	@echo ""
	@# ── 生成 .env ──
	@_TAG=$$(git describe --tags --abbrev=0 2>/dev/null); \
	_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null); \
	_BUILD_TIME=$$(date -u '+%Y-%m-%dT%H:%M:%SZ'); \
	echo "# Auto-generated by 'make docker-release'" > .env; \
	echo "MINDX_VERSION=$$_TAG" >> .env; \
	echo "MINDX_COMMIT=$$_COMMIT" >> .env; \
	echo "MINDX_BUILD_TIME=$$_BUILD_TIME" >> .env; \
	echo "$(GREEN)📝 .env generated → MINDX_VERSION=$$_TAG$(NC)"
	@echo ""
	@# ── 构建镜像 ──
	@_TAG=$$(git describe --tags --abbrev=0 2>/dev/null); \
	_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null); \
	_BUILD_TIME=$$(date -u '+%Y-%m-%dT%H:%M:%SZ'); \
	echo "$(GREEN)🐳 Building Docker image $(DOCKER_USER)/$(BINARY_NAME):$$_TAG ...$(NC)"; \
	docker build \
		-t $(DOCKER_USER)/$(BINARY_NAME):$$_TAG \
		-t $(DOCKER_USER)/$(BINARY_NAME):latest \
		--build-arg VERSION=$$_TAG \
		--build-arg COMMIT=$$_COMMIT \
		--build-arg BUILD_TIME="$$_BUILD_TIME" \
		. && \
	echo "$(GREEN)✅ Image built: $(DOCKER_USER)/$(BINARY_NAME):$$_TAG$(NC)" && \
	echo "" && \
	echo "$(GREEN)📤 Pushing to Docker Hub...$(NC)" && \
	docker push $(DOCKER_USER)/$(BINARY_NAME):$$_TAG && \
	docker push $(DOCKER_USER)/$(BINARY_NAME):latest && \
	echo "" && \
	echo "$(GREEN)═══════════════════════════════════════════════════════════════$(NC)" && \
	echo "$(GREEN)  🎉 Docker Release Complete!$(NC)" && \
	echo "$(GREEN)═══════════════════════════════════════════════════════════════$(NC)" && \
	echo "" && \
	echo "  Image:    $(CYAN)$(DOCKER_USER)/$(BINARY_NAME):$$_TAG$(NC)" && \
	echo "  Latest:   $(CYAN)$(DOCKER_USER)/$(BINARY_NAME):latest$(NC)" && \
	echo "  Pull:     $(CYAN)docker pull $(DOCKER_USER)/$(BINARY_NAME):$$_TAG$(NC)" && \
	echo ""

## docker-clean: 清理 Docker 资源
docker-clean:
	@echo "$(RED)🧹 Cleaning Docker resources...$(NC)"
	-docker compose down --remove-orphans 2>/dev/null || true
	-docker stop $(BINARY_NAME)-tui $(BINARY_NAME)-daemon 2>/dev/null || true
	-docker rm $(BINARY_NAME)-tui $(BINARY_NAME)-daemon 2>/dev/null || true
	-docker rmi $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest 2>/dev/null || true
	@echo "$(GREEN)✅ Docker cleanup complete!$(NC)"

# =============================================================================
# CI/CD 目标
# =============================================================================

## CI: 完整的 CI 流水线（test + build）
ci: test build
	@echo "$(GREEN)✅ CI pipeline completed successfully!$(NC)"

## CD: 完整的 CD 流水线（CI + release + docker-build）
cd: ci release docker-build
	@echo "$(GREEN)✅ CD pipeline completed successfully!$(NC)"

## pre-commit: Git pre-commit hook（快速测试）
pre-commit: test
	@echo "$(GREEN)✅ Pre-commit checks passed!$(NC)"

# =============================================================================
# 信息目标
# =============================================================================

## version: 显示版本和构建信息
version:
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo "$(GREEN)  $(BOLD)MindX Build Information$(NC)"
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo "$(CYAN)  Version:      $(NC)$(VERSION)"
	@echo "$(CYAN)  Go Version:   $(NC)$(GOVERSION)"
	@echo "$(CYAN)  Binary Name:  $(NC)$(BINARY_NAME)"
	@echo "$(CYAN)  Build Time:   $(NC)$(BUILD_TIME)"
	@echo "$(CYAN)  Git Commit:   $(NC)$(GIT_COMMIT)"
	@echo "$(CYAN)  Working Tree: $(NC)$(GIT_DIRTY)"
	@echo "$(CYAN)  LDFLAGS:      $(NC)$(LDFLAGS)"
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"

## info: 显示项目详细信息
info:
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo "$(GREEN)  $(BOLD)MindX Project Info$(NC)"
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo "$(CYAN)Binary:    $(NC)$(BINARY_NAME)"
	@echo "$(CYAN)Go:        $(NC)$($(GO) version)"
	@echo ""
	@echo "$(CYAN)Modules:$(NC)"
	@$(GO) list -m all 2>/dev/null | head -15
	@echo ""
	@echo "$(CYAN)Statistics:$(NC)"
	@echo "  Go files:     $$(find . -name '*.go' -not -path './vendor/*' -not -path './.git/*' | wc -l | tr -d ' ')"
	@echo "  Total size:   $$(du -sh . | cut -f1)"
	@echo "  Test files:   $$(find . -name '*_test.go' -not -path './vendor/*' | wc -l | tr -d ' ')"
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"

## help: 显示详细的帮助信息
help:
	@echo ""
	@echo "$(GREEN)╔══════════════════════════════════════════════╗$(NC)"
	@echo "$(GREEN)║     $(BOLD)MindX Build System - Complete Reference$(NC)     $(GREEN)║$(NC)"
	@echo "$(GREEN)╚══════════════════════════════════════════════╝$(NC)"
	@echo ""
	@echo "$(YELLOW)📦 Build Targets:$(NC)"
	@echo "  $(GREEN)build$(NC)           Compile for current platform"
	@echo "  $(GREEN)build-all$(NC)      Build all platforms (darwin + linux + windows)"
	@echo "  $(GREEN)build-debug$(NC)     Compile debug binary (with symbols)"
	@echo "  $(GREEN)setup-cross$(NC)     Install cross-compilation toolchains (brew)"
	@echo "  $(GREEN)build-linux-amd64$(NC)   Linux x86_64"
	@echo "  $(GREEN)build-linux-arm64$(NC)   Linux ARM64"
	@echo "  $(GREEN)build-darwin-amd64$(NC)   macOS Intel"
	@echo "  $(GREEN)build-darwin-arm64$(NC)   macOS Apple Silicon"
	@echo "  $(GREEN)build-windows-amd64$(NC)  Windows x86_64"
	@echo "  $(GREEN)install$(NC)         Install to system PATH (requires sudo)"
	@echo "  $(GREEN)uninstall$(NC)       Remove from system PATH"
	@echo ""
	@echo "$(YELLOW)▶️ Run & Dev Targets:$(NC)"
	@echo "  $(GREEN)run$(NC)             Build and start TUI"
	@echo "  $(GREEN)run-daemon$(NC)      Build and start Daemon service"
	@echo "  $(GREEN)restart$(NC)         Build and restart daemon via launchd/systemd (non-blocking)"
	@echo "  $(GREEN)stop$(NC)             Stop running daemon"
	@echo "  $(GREEN)dev$(NC)             Go run (no build, for development)"
	@echo "  $(GREEN)dev-watch$(NC)      File watcher auto-reload (air)"
	@echo ""
	@echo "$(YELLOW)🧪 Test Targets:$(NC)"
	@echo "  $(GREEN)test$(NC)            Run unit tests with coverage"
	@echo "  $(GREEN)test-verbose$(NC)     Verbose test output"
	@echo ""
	@echo "$(YELLOW)📝 Documentation Targets:$(NC)"
	@echo "  $(GREEN)docs$(NC)            Generate API documentation"
	@echo "  $(GREEN)docs-serve$(NC)      Start local doc server"
	@echo "  $(GREEN)changelog$(NC)       Generate CHANGELOG.md"
	@echo ""
	@echo "$(YELLOW)🚀 Release Targets:$(NC)"
	@echo "$(GREEN)  $(GREEN)release$(NC)          Create release packages"
	@echo "  $(GREEN)release-notes$(NC)    Generate release notes"
	@echo ""
	@echo "$(YELLOW)🐳 Docker Targets:$(NC)"
	@echo "  $(GREEN)docker$(NC)            Full pipeline: git version → .env → build → up"
	@echo "  $(GREEN).env$(NC)              Generate .env from git tag (no build)"
	@echo "  $(GREEN)docker-build$(NC)     Build Docker image (custom VERSION=)"
	@echo "  $(GREEN)docker-release$(NC)   Local build + push to Docker Hub (version = git tag)"
	@echo "  $(GREEN)docker-run$(NC)       Run container (TUI mode)"
	@echo "  $(GREEN)docker-run-daemon$(NC) Run container (daemon mode, 1313+1314)"
	@echo "  $(GREEN)docker-push$(NC)      Push image to registry"
	@echo "  $(GREEN)docker-clean$(NC)     Clean up Docker resources"
	@echo ""
	@echo "$(YELLOW)🔄 CI/CD Targets:$(NC)"
	@echo "  $(GREEN)ci$(NC)               Full CI pipeline"
	@echo "  $(GREEN)cd$(NC)               Full CD pipeline"
	@echo "  $(GREEN)pre-commit$(NC)       Git pre-commit hook"
	@echo ""
	@echo "$(YELLOW)🔍 Code Quality Gates (run before every build):$(NC)"
	@echo "  $(GREEN)fmt$(NC)              Check & auto-fix Go formatting (gofmt)"
	@echo "  $(GREEN)lint$(NC)             Run golint code analysis"
	@echo ""
	@echo "$(YELLOW)ℹ️ Info Targets:$(NC)"
	@echo "  $(GREEN)version$(NC)          Show version information"
	@echo "  $(GREEN)info$(NC)             Show project statistics"
	@echo "  $(GREEN)help$(NC)             Show this help message"
	@echo ""
	@echo "$(YELLOW)🧹 Utility Targets:$(NC)"
	@echo "  $(GREEN)clean$(NC)            Remove all build artifacts"
	@echo "  $(GREEN)clear$(NC)            ⚠ Delete dist/, tmp/, and entire ~/.mindx workspace"
	@echo ""
	@echo "$(CYAN)Examples:$(NC)"
	@echo "  make build && make run                    # Build and run TUI"
	@echo "  make test                                 # Run tests"
	@echo "  make ci                                   # Full CI pipeline"
	@echo "  make release && make release-notes         # Create release"
	@echo "  make build                                 # Multi-platform build"
	@echo "  make docker-build && make docker-run       # Docker workflow"
	@echo ""

# =============================================================================
# 清理目标
# =============================================================================

## clean: 清理所有构建产物和临时文件
clean:
	@echo "$(RED)🧹 Cleaning build artifacts...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -rf $(COVERAGE_DIR)
	rm -f coverage.out
	rm -f profile.out
	rm -f cpu.prof
	rm -f mem.prof
	rm -rf releases/
	@echo "$(GREEN)✅ Clean complete! Ready for fresh build.$(NC)"

## clean-all: 彻底清理（包括 vendor、依赖缓存等）
clean-all: clean
	@echo "$(RED)🧹 Deep cleaning...$(NC)"
	rm -rf vendor/
	rm -rf $(DOCS_DIR)
	@echo "$(GREEN)✅ Deep clean complete!$(NC)"

## clear: 清理构建产物、临时文件与 mindx 工作区（危险！会删除 ~/.mindx）
clear:
	@echo "$(RED)☢️  WARNING: This will delete entire ~/.mindx workspace!$(NC)"
	@echo "$(YELLOW)  Includes: all agents, settings, sessions, logs, memory$(NC)"
	@read -p "Type 'yes' to confirm: " reply; \
	if [ "$$reply" = "yes" ]; then \
		rm -rf $(BUILD_DIR) ./tmp ~/.mindx; \
		echo "$(GREEN)✅ Clear complete! dist/, tmp/, ~/.mindx removed.$(NC)"; \
	else \
		echo "$(RED)❌ Aborted.$(NC)"; \
	fi

## clear-creds: 清空密钥存储中所有 provider API Key（macOS Keychain 或 Linux 加密文件）
clear-creds:
	@echo "$(YELLOW)🔑 Clearing provider credentials...$(NC)"
	@_PROVIDER_KEYS="DASHSCOPE_API_KEY DEEPSEEK_API_KEY ZHIPU_API_KEY MOONSHOT_API_KEY MINIMAX_API_KEY ARK_API_KEY OPENAI_API_KEY ANTHROPIC_API_KEY GEMINI_API_KEY"; \
	if [ "$$(uname -s)" = "Darwin" ]; then \
		echo "$(CYAN)   Platform: macOS (Keychain)$(NC)"; \
		for _key in $$_PROVIDER_KEYS; do \
			if security find-generic-password -s mindx -a "$$_key" -w >/dev/null 2>&1; then \
				security delete-generic-password -s mindx -a "$$_key" >/dev/null 2>&1 && \
					echo "     $(GREEN)✓ Deleted: $$_key$(NC)" || \
					echo "     $(YELLOW)⚠ Failed to delete: $$_key$(NC)"; \
			else \
				echo "     - Skipped (not found): $$_key"; \
			fi; \
		done; \
	else \
		echo "$(CYAN)   Platform: Linux (encrypted file)$(NC)"; \
		_CRED_FILE="$${HOME}/.mindx/settings/.credentials"; \
		if [ -f "$$_CRED_FILE" ]; then \
			rm -f "$$_CRED_FILE" && echo "     $(GREEN)✓ Removed: $$_CRED_FILE$(NC)" || \
				echo "     $(RED)✗ Failed to remove credentials file$(NC)"; \
		else \
			echo "     - No credentials file found"; \
		fi; \
	fi; \
	echo "$(GREEN)✅ Provider credentials cleared.$(NC)"

# =============================================================================
# 内部目标（辅助功能）
# =============================================================================

pre-build:
	@echo "$(YELLOW)⏳ Pre-build checks...$(NC)"
	@test -n "$(GO)" || (echo "$(RED)❌ Error: Go not found$(NC)" && exit 1)
	@$(GO) version >/dev/null 2>&1 && echo "$(GREEN)✅ Pre-build checks passed!$(NC)" || (echo "$(YELLOW)⚠  Go version check warning (continuing)...$(NC)" && echo "$(GREEN)✅ Pre-build checks passed!$(NC)")

# =============================================================================
# 默认目标
# =============================================================================

.DEFAULT_GOAL := help

# =============================================================================
# 特殊目标（Git hooks 配置示例）
# =============================================================================

## setup-hooks: 配置 Git hooks（可选）
setup-hooks:
	@echo "$(GREEN)🪝 Setting up Git hooks...$(NC)"
	@mkdir -p .git/hooks
	@echo '#!/bin/bash' > .git/hooks/pre-commit
	@echo 'make pre-commit' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "$(GREEN)✅ Git hooks configured!$(NC)"
	@echo "$(CYAN)Pre-commit hook will run: make pre-commit$(NC)"

# =============================================================================
# end of Makefile
# =============================================================================
