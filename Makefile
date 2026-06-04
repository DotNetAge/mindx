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
#   make test              # 运行所有测试
#   make bench             # 性能基准测试
#   make lint              # 代码检查
#   make clean             # 清理构建产物
#   make docs              # 生成文档
#   make tidy              # 整理依赖
#   make release           # 发布多平台版本
#   make docker-build      # 构建 Docker 镜像
#
# =============================================================================

.PHONY: build build-current setup-cross clear install run run-daemon stop test bench lint clean docs tidy help \
        dev dev-tui dev-daemon uninstall format check vet \
        release release-notes release-publish release-homebrew release-winget publish \
        cross-build docker-build docker-push \
        ci cd security audit deps-update \
        pre-commit post-commit version info changelog

# =============================================================================
# 配置变量
# =============================================================================

# 项目信息
BINARY_NAME    ?= mindx
PROJECT_NAME   ?= mindx
VERSION        ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v2.1.0")
BUILD_TIME     ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY      ?= $(shell git diff --quiet HEAD 2>/dev/null && echo "clean" || echo "dirty")

# Go 工具链
GO             ?= go
GOFMT          ?= gofmt
GOIMPORTS      ?= goimports
GOVET          ?= go vet
GOLINT         ?= golangci-lint

# Go 版本信息
GOVERSION      ?= $(shell $(GO) version | grep -oP 'go\d+\.\d+' | head -1)

# 版本号（不含 v 前缀）
VERSION_NUM    := $(VERSION:v%=%)

# 构建标志
LDFLAGS        ?= -s -w \
                 -X main.version=$(VERSION_NUM) \
                 -X main.commit=$(GIT_COMMIT) \
                 -X main.buildTime=$(BUILD_TIME) \
                 -X main.dirty=$(GIT_DIRTY)

GOFLAGS        ?= -trimpath -ldflags "$(LDFLAGS)"

# 目录配置
BUILD_DIR      ?= ./dist
DIST_DIR       ?= ./dist
COVERAGE_DIR   ?= ./coverage
BENCHMARK_DIR  ?= .benchmarks
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
# 主要构建目标
# =============================================================================

## build: 编译 macOS、Linux、Windows 三平台二进制至 dist/
build: pre-build
	@mkdir -p $(BUILD_DIR)
	@echo "$(GREEN)➡ Building darwin/amd64...$(NC)"
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "$(GREEN)  ✅ darwin/amd64$(NC)"
	@echo "$(GREEN)➡ Building linux/amd64...$(NC)"
	@if command -v x86_64-linux-musl-gcc >/dev/null 2>&1; then \
		CGO_ENABLED=1 CC=x86_64-linux-musl-gcc GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 . && \
		echo "$(GREEN)  ✅ linux/amd64$(NC)"; \
	else \
		echo "$(YELLOW)  ⚠  linux/amd64 skipped — install: brew install FiloSottile/musl-cross/musl-cross$(NC)"; \
	fi
	@echo "$(GREEN)➡ Building windows/amd64...$(NC)"
	@if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe . && \
		echo "$(GREEN)  ✅ windows/amd64$(NC)"; \
	else \
		echo "$(YELLOW)  ⚠  windows/amd64 skipped — install: brew install mingw-w64$(NC)"; \
	fi
	@echo "$(GREEN)✅ Build complete!$(NC)"
	@ls -lh $(BUILD_DIR)/

## build-current: 仅编译当前平台（供 run/install 使用）
build-current: pre-build
	@echo "$(GREEN)➡ Building $(BINARY_NAME) v$(VERSION_NUM) for $(shell $(GO) env GOOS)/$(shell $(GO) env GOARCH)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "$(GREEN)✅ Build complete!$(NC)"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)

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
	@echo "$(GREEN)✅ Cross-compilation toolchains installed! Run 'make build' for all platforms.$(NC)"

## build-debug: 编译调试版本（带符号信息）
build-debug:
	@echo "$(YELLOW)🔧 Building debug binary...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug .
	@echo "$(GREEN)✅ Debug build complete: $(BUILD_DIR)/$(BINARY_NAME)-debug$(NC)"

## install: 构建并部署到 ~/.mindx（含 runtime 资源 + PATH + 系统服务配置）
install:
	@echo "$(GREEN)➡ Building $(BINARY_NAME) → ~/.mindx/bin/$(BINARY_NAME)...$(NC)"
	@mkdir -p ~/.mindx/bin ~/.mindx/settings
	@rm -f ~/.mindx/bin/$(BINARY_NAME)
	@CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o ~/.mindx/bin/$(BINARY_NAME) . && \
		echo "$(GREEN)  ✅ $(BINARY_NAME) → ~/.mindx/bin/$(BINARY_NAME)$(NC)"
	@echo "$(GREEN)➡ Copying runtime files...$(NC)"
	@cp -r runtime/* ~/.mindx/ && \
		echo "$(GREEN)  ✅ runtime/ → ~/.mindx/$(NC)"
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
		PLIST="$$HOME/.mindx/settings/com.dotnetage.$(BINARY_NAME).plist"; \
		LABEL="com.dotnetage.$(BINARY_NAME)"; \
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
			'        <string>start</string>' \
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
		echo "$(CYAN)  ⟳ Stopping existing service...$(NC)" && \
		launchctl unload "$$LAUNCH_AGENTS/$$LABEL.plist" 2>/dev/null && echo "     stopped" || echo "     (not running)"; \
		echo "$(CYAN)  ⟳ Starting service...$(NC)" && \
		launchctl load "$$LAUNCH_AGENTS/$$LABEL.plist" && \
		echo "$(GREEN)  ✅ Daemon registered and started$(NC)"; \
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

## uninstall: 卸载已安装的二进制文件
uninstall:
	@echo "$(RED)🗑️ Uninstalling $(BINARY_NAME)...$(NC)"
	@rm -f /usr/local/bin/$(BINARY_NAME) 2>/dev/null || true
	@rm -f $$GOPATH/bin/$(BINARY_NAME) 2>/dev/null || true
	@echo "$(GREEN)✅ Successfully uninstalled!$(NC)"

# =============================================================================
# 运行目标
# =============================================================================

## run: 编译并启动 TUI（默认模式）
run: build-current
	@echo "$(YELLOW)🚀 Starting TUI (Terminal UI)...$(NC)"
	@echo "$(CYAN)💡 Tips:$(NC)"
	@echo "  • Enter messages and press Enter to send"
	@echo "  • Type /help for available commands"
	@echo "  • Press Ctrl+C to exit"
	@echo ""
	./$(BUILD_DIR)/$(BINARY_NAME)

## run-daemon: 编译并启动 Daemon 服务
run-daemon: build-current
	@echo "$(YELLOW)🔧 Starting Daemon service...$(NC)"
	@echo "$(CYAN)💡 Service info:$(NC)"
	@echo "  • WebSocket: ws://localhost:1314/ws"
	@echo "  • Press Ctrl+C to stop"
	@echo ""
	./$(BUILD_DIR)/$(BINARY_NAME) start

## stop: 停止本机 mindx daemon（由 make install 启动的守护进程）
stop:
	@echo "$(YELLOW)🛑 Stopping mindx daemon...$(NC)"
	@pkill -f "mindx start" 2>/dev/null || \
	 pkill -f "$(BINARY_NAME) start" 2>/dev/null || \
	 (lsof -ti:1313 -ti:1314 | xargs kill 2>/dev/null) || \
	 echo "$(GREEN)  ✅ No running daemon found.$(NC)"
	@sleep 1
	@echo "$(GREEN)✅ mindx daemon stopped.$(NC)"

## run-verbose: 以详细日志模式运行 TUI
run-verbose: build-current
	@echo "$(YELLOW)🚀 Starting TUI (verbose mode)...$(NC)"
	MINDX_LOG_LEVEL=debug ./$(BUILD_DIR)/$(BINARY_NAME)

# =============================================================================
# 开发目标
# =============================================================================

## dev: 开发模式（TUI + 热重载，推荐日常开发）
dev:
	@echo "$(YELLOW)🛠️  Running in development mode...$(NC)"
	@echo "$(CYAN)📝 Using: go run . (auto-reload on file changes)$(NC)"
	$(GO) run .

## dev-tui: 仅运行 TUI（不重新编译，快速测试）
dev-tui:
	@echo "$(YELLOW)🚀 Quick TUI run...$(NC)"
	$(GO) run cmd/root.go

## dev-daemon: 仅运行 Daemon（不重新编译，快速测试）
dev-daemon:
	@echo "$(YELLOW)🔧 Quick Daemon run...$(NC)"
	$(GO) run cmd/start.go

## dev-watch: 文件监控自动重载（需要 air 或 CompileDaemon）
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

## test-short: 快速测试（跳过慢速和集成测试）
test-short:
	@echo "$(GREEN)▶ Running short tests (skip slow/integration)...$(NC)"
	$(GO) test -short -v ./...

## test-integration: 运行集成测试（需要完整环境）
test-integration:
	@echo "$(GREEN)▶ Running integration tests...$(NC)"
	$(GO) test -run Integration -v ./internal/svc/...

## test-race: 竞态条件检测
test-race:
	@echo "$(GREEN)▶ Running race detector...$(NC)"
	$(GO) test -race -v ./...

## test-verbose: 详细测试输出（包含所有包）
test-verbose:
	@echo "$(GREEN)▶ Running all tests with verbose output...$(NC)"
	$(GO) test -v ./...

## test-specific: 运行特定测试函数
# 用法: make test-specific TESTFUNC=TestDefaultApp
test-specific:
	@echo "$(GREEN)▶ Running specific test: $(TESTFUNC)...$(NC)"
	$(GO) test -run $(TESTFUNC) -v ./...

## bench: 性能基准测试
bench:
	@echo "$(GREEN)⏱ Running benchmarks...$(NC)"
	@mkdir -p $(BENCHMARK_DIR)
	$(GO) test -bench=. -benchmem -count=1 ./internal/core/... > $(BENCHMARK_DIR)/bench-$(shell date +%Y%m%d-%H%M%S).txt
	@echo "$(GREEN)✅ Benchmarks saved to $(BENCHMARK_DIR)/$(NC)"

## bench-compare: 对比两次基准测试结果
# 用法: make bench-compare OLD=bench-20260510-120000.txt NEW=bench-20260510-130000.txt
bench-compare:
	@echo "$(GREEN)📊 Comparing benchmarks...$(NC)"
	@if [ -z "$(OLD)" ] || [ -z "$(NEW)" ]; then \
		echo "$(RED)❌ Error: OLD and NEW parameters required$(NC)"; \
		echo "$(YELLOW)Usage: make bench-compare OLD=file1.txt NEW=file2.txt$(NC)"; \
		exit 1; \
	fi
	benchstat $(BENCHMARK_DIR)/$(OLD) $(BENCHMARK_DIR)/$(NEW)

## bench-save: 保存当前基准结果用于后续对比
bench-save: bench
	@echo "$(GREEN)💾 Benchmarks saved successfully!$(NC)"

# =============================================================================
# 代码质量目标
# =============================================================================

## lint: 运行 golangci-lint（全面的代码质量检查）
lint:
	@echo "$(GREEN)🔍 Running linters...$(NC)"
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run ./... ; \
	else \
		echo "$(YELLOW)⚠ $(GOLINT) not installed.$(NC)"; \
		echo "$(CYAN)Install: brew install golangci-lint  # macOS$(NC)"; \
		echo "$(CYAN)        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
	fi

## lint-fix: 自动修复可修复的 lint 问题
lint-fix:
	@echo "$(GREEN)🔧 Auto-fixing lint issues...$(NC)"
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run --fix ./... ; \
	else \
		echo "$(YELLOW)⚠ $(GOLINT) not installed. See 'make lint' for installation instructions.$(NC)"; \
	fi

## vet: Go vet 静态分析
vet:
	@echo "$(GREEN)🔍 Running go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)✅ Vet passed! No issues found.$(NC)"

## fmt: 格式化所有 Go 代码
fmt:
	@echo "$(GREEN)🎨 Formatting code...$(NC)"
	@find . -name "*.go" -not -path "./vendor/*" -exec $(GOFMT) -w {} \;
	@echo "$(GREEN)✅ All files formatted!$(NC)"

## fmt-check: 检查代码格式是否正确（不修改文件）
check:
	@echo "$(GREEN)🔍 Checking code formatting...$(NC)"
	@test -z "$$($(GOFMT) -l ./... | grep -v vendor)" || { \
		echo "$(YELLOW)⚠ Files need formatting:$(NC)"; \
		$(GOFMT) -l ./... | grep -v vendor; \
		echo "$(CYAN)Run 'make fmt' to fix automatically.$(NC)"; \
		exit 1; \
	}
	@echo "$(GREEN)✅ All files properly formatted!$(NC)"

## imports: 整理 import 顺序和移除未使用的导入
imports:
	@echo "$(Green)📦 Organizing imports...$(NC)"
	@if command -v $(GOIMPORTS) >/dev/null 2>&1; then \
		find . -name "*.go" -not -path "./vendor/*" -exec $(GOIMPORTS) -w {} \; ; \
	else \
		echo "$(YELLOW)⚠ goimports not installed.$(NC)"; \
		echo "$(CYAN)Install: go install golang.org/x/tools/cmd/goimports@latest$(NC)"; \
	fi

# =============================================================================
# 安全与审计目标
# =============================================================================

## security: 安全漏洞扫描
security:
	@echo "$(GREEN)🔒 Running security scan...$(NC)"
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./... ; \
	else \
		echo "$(YELLOW)⚠ govulncheck not installed.$(NC)"; \
		echo "$(CYAN)Install: go install golang.org/x/vuln/cmd/govulncheck@latest$(NC)"; \
	fi

## audit: 完整的依赖审计
audit: security
	@echo "$(GREEN)📋 Auditing dependencies...$(NC)"
	$(GO) list -m all | head -20
	@echo ""
	@echo "$(GREEN)✅ Audit complete!$(NC)"

## vulnerability-check: 检查已知 CVE 漏洞
vulnerability-check:
	@echo "$(RED)🛡️ Checking for known vulnerabilities...$(NC)"
	@if command -v snyk >/dev/null 2>&1; then \
		snyk test ; \
	elif command -v trivy >/dev/null 2>&1; then \
		trivy fs --severity HIGH,CRITICAL . ; \
	else \
		echo "$(YELLOW)⚠ No vulnerability scanner found.$(NC)"; \
		echo "$(CYAN)Recommended: snyk or trivy$(NC)"; \
	fi

# =============================================================================
# 依赖管理
# =============================================================================

## tidy: 整理 Go 模块依赖（清理未使用的依赖）
tidy:
	@echo "$(GREEN)📦 Tidying modules...$(NC)"
	$(GO) mod tidy
	$(GO) mod verify
	@echo "$(GREEN)✅ Modules tidied and verified!$(NC)"

## deps: 下载所有依赖到本地缓存
deps:
	@echo "$(GREEN)📦 Downloading dependencies...$(NC)"
	$(GO) mod download
	@echo "$(GREEN)✅ Dependencies ready!$(NC)"

## deps-update: 更新所有依赖到最新版本（谨慎使用）
deps-update:
	@echo "$(YELLOW)⚠ Updating all dependencies...$(NC)"
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "$(GREEN)✅ Dependencies updated! Review changes before committing.$(NC)"

## deps-graph: 显示依赖关系图
deps-graph:
	@echo "$(BLUE)📊 Dependency graph:$(NC)"
	$(GO) mod graph | head -50

## vendor: 创建 vendor 目录（用于离线构建）
vendor:
	@echo "$(GREEN)📦 Creating vendor directory...$(NC)"
	$(GO) mod vendor
	@echo "$(GREEN)✅ Vendor directory created!$(NC)"

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
	pkgsite http://localhost:6060

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
	@if command -l git-chglog >/dev/null 2>&1; then \
		git-chglog -o CHANGELOG.md ; \
	else \
		git log --pretty=format:"- %s (%h)" --since="1 month ago" > CHANGELOG.md.tmp ; \
		mv CHANGELOG.md.tmp CHANGELOG.md ; \
	fi
	@echo "$(GREEN)✅ Changelog generated!$(NC)"

# =============================================================================
# 交叉编译目标（多平台支持）
# =============================================================================

## cross-build: 编译所有主流平台版本
cross-build: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64
	@echo "$(GREEN)✅ All platforms built! Check $(DIST_DIR)/$(NC)"

## build-linux-amd64: Linux x86_64
build-linux-amd64:
	@echo "$(GREEN)➡ Building for linux/amd64...$(NC)"
	@mkdir -p $(DIST_DIR)
	@if command -v x86_64-linux-musl-gcc >/dev/null 2>&1; then \
		CGO_ENABLED=1 CC=x86_64-linux-musl-gcc GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 . && \
		echo "$(GREEN)✅ linux/amd64 done$(NC)"; \
	else \
		echo "$(YELLOW)⚠  linux/amd64 skipped — install: brew install FiloSottile/musl-cross/musl-cross$(NC)"; \
	fi

## build-linux-arm64: Linux ARM64
build-linux-arm64:
	@echo "$(GREEN)➡ Building for linux/arm64...$(NC)"
	@mkdir -p $(DIST_DIR)
	@if command -v aarch64-linux-musl-gcc >/dev/null 2>&1; then \
		CGO_ENABLED=1 CC=aarch64-linux-musl-gcc GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 . && \
		echo "$(GREEN)✅ linux/arm64 done$(NC)"; \
	else \
		echo "$(YELLOW)⚠  linux/arm64 skipped — install: brew install FiloSottile/musl-cross/musl-cross$(NC)"; \
	fi

## build-darwin-amd64: macOS Intel
build-darwin-amd64:
	@echo "$(GREEN)➡ Building for darwin/amd64...$(NC)"
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "$(GREEN)✅ darwin/amd64 done$(NC)"

## build-darwin-arm64: macOS Apple Silicon (M1/M2)
build-darwin-arm64:
	@echo "$(GREEN)➡ Building for darwin/arm64 (Apple Silicon)...$(NC)"
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "$(GREEN)✅ darwin/arm64 done$(NC)"

## build-windows-amd64: Windows x86_64
build-windows-amd64:
	@echo "$(GREEN)➡ Building for windows/amd64...$(NC)"
	@mkdir -p $(DIST_DIR)
	@if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		if CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe . 2>/dev/null; then \
			echo "$(GREEN)✅ windows/amd64 done (CGO)$(NC)"; \
		else \
			echo "$(YELLOW)⚠  windows/amd64 CGO build failed, trying CGO_ENABLED=0...$(NC)"; \
			CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe . && \
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
	@binary="$(DIST_DIR)/$(BINARY_NAME)-$(1)-$(2)$(3)"; \
	if [ ! -f "$$binary" ]; then \
		echo "  ⚠  $$(basename $$binary) not found, skipping"; \
		exit 0; \
	fi; \
	archive="$(REL)/$(BINARY_NAME)-$(V)-$(1)-$(2).tar.gz"; \
	if [ "$(1)" = "windows" ]; then \
		archive="$(REL)/$(BINARY_NAME)-$(V)-$(1)-$(2).zip"; \
		cp "$$binary" "$(DIST_DIR)/$(BINARY_NAME).exe"; \
		(cd "$(DIST_DIR)" && zip -q "$$OLDPWD/$$archive" "$(BINARY_NAME).exe"); \
		rm -f "$(DIST_DIR)/$(BINARY_NAME).exe"; \
	else \
		cp "$$binary" "$(DIST_DIR)/$(BINARY_NAME)"; \
		tar czf "$$archive" -C "$(DIST_DIR)" "$(BINARY_NAME)"; \
		rm -f "$(DIST_DIR)/$(BINARY_NAME)"; \
	fi; \
	echo "  ✅ $$(basename $$archive)  $$(ls -lh $$archive | awk '{print $$5}')"
endef

## release: 交叉编译并打包所有平台（带 checksums）
release: clean cross-build
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

## release-homebrew: 生成并推送 Homebrew formula
release-homebrew:
	@echo "$(GREEN)🍺 Generating Homebrew formula...$(NC)"
	@SHA256_AMD64=$$(shasum -a 256 "$(REL)/$(BINARY_NAME)-$(V)-darwin-amd64.tar.gz" 2>/dev/null | cut -d' ' -f1); \
	SHA256_ARM64=$$(shasum -a 256 "$(REL)/$(BINARY_NAME)-$(V)-darwin-arm64.tar.gz" 2>/dev/null | cut -d' ' -f1); \
	if [ -z "$$SHA256_AMD64" ] || [ -z "$$SHA256_ARM64" ]; then \
		echo "$(RED)❌ darwin tarballs not found in $(REL)/$(NC)"; \
		exit 1; \
	fi; \
	TAP_DIR=$$(mktemp -d); \
	git clone --depth=1 "https://github.com/$(HOMEBREW_TAP).git" "$$TAP_DIR" 2>/dev/null || { \
		echo "$(YELLOW)⚠  Cannot clone $(HOMEBREW_TAP). Formula saved locally.$(NC)"; \
		mkdir -p "$$TAP_DIR/Formula"; \
	}; \
	cat > "$$TAP_DIR/Formula/$(BINARY_NAME).rb" <<-FORMULA
	# typed: false
	# frozen_string_literal: true

	class Mindx < Formula
	  desc "MindX - AI-native multi-agent conversation platform"
	  homepage "https://github.com/$(GITHUB_REPO)"
	  license "MIT"
	  version "$(V)"

	  on_macos do
	    if Hardware::CPU.intel?
	      url "https://github.com/$(GITHUB_REPO)/releases/download/v$(V)/$(BINARY_NAME)-$(V)-darwin-amd64.tar.gz"
	      sha256 "$${SHA256_AMD64}"
	    end

	    if Hardware::CPU.arm?
	      url "https://github.com/$(GITHUB_REPO)/releases/download/v$(V)/$(BINARY_NAME)-$(V)-darwin-arm64.tar.gz"
	      sha256 "$${SHA256_ARM64}"
	    end
	  end

	  def install
	    bin.install "$(BINARY_NAME)"
	  end

	  test do
	    assert_match "MindX", shell_output("\#{bin}/$(BINARY_NAME) --help")
	  end
	end
	FORMULA; \
	if [ -d "$$TAP_DIR/.git" ]; then \
		cd "$$TAP_DIR" && git add -A && git commit -m "$(BINARY_NAME) v$(V)" && git push; \
		echo "$(GREEN)✅ Homebrew tap updated: $(HOMEBREW_TAP)$(NC)"; \
	else \
		cp "$$TAP_DIR/Formula/$(BINARY_NAME).rb" "$(REL)/$(BINARY_NAME)-$(V).rb"; \
		echo "$(GREEN)✅ Formula saved: $(REL)/$(BINARY_NAME)-$(V).rb$(NC)"; \
	fi; \
	rm -rf "$$TAP_DIR"

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
	@# ── Docker ──
	@read -p "  推送 Docker 镜像? [y/N]: " docker_confirm; \
	if [ "$$docker_confirm" = "y" ] || [ "$$docker_confirm" = "Y" ]; then \
		$(MAKE) docker-build docker-push; \
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

## docker: 开发指令 — 编译 Linux 二进制 + 打包 runtime/bin/ + Docker Compose 构建并启动
docker: build-linux-amd64
	@echo "$(GREEN)📦 Copying binary to runtime/bin/mindx...$(NC)"
	cp $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 runtime/bin/mindx
	@echo "$(GREEN)🐳 Starting MindX via Docker Compose...$(NC)"
	docker compose build
	@echo "$(GREEN)✅ Build complete. Starting daemon...$(NC)"
	docker compose up -d
	@echo "$(GREEN)✅ MindX daemon is running!$(NC)"
	@echo "   Web UI:  $(CYAN)http://localhost:1313$(NC)"
	@echo "   WebSocket: $(CYAN)ws://localhost:1314$(NC)"
	@echo "$(YELLOW)   Run 'docker compose logs -f' to tail logs$(NC)"
	@echo "$(YELLOW)   Run 'docker compose down' to stop$(NC)"

## docker-build: 构建 Docker 镜像
docker-build:
	@echo "$(GREEN)🐳 Building Docker image...$(NC)"
	docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .
	@echo "$(GREEN)✅ Docker image built: $(BINARY_NAME):$(VERSION)$(NC)"

## docker-run: 运行 Docker 容器（TUI 模式）
docker-run:
	@echo "$(YELLOW)🐳 Running Docker container (TUI mode)...$(NC)"
	docker run -it --rm \
		-v ~/.mindx:/root/.mindx \
		--name $(BINARY_NAME)-tui \
		$(BINARY_NAME):latest

## docker-run-daemon: 运行 Docker 容器（Daemon 模式）
docker-run-daemon:
	@echo "$(YELLOW)🐳 Running Docker container (Daemon mode)...$(NC)"
	docker run -d \
		--name $(BINARY_NAME)-daemon \
		-p 1314:1314 \
	-v ~/.mindx:/root/.mindx \
		$(BINARY_NAME):latest start

## docker-push: 推送 Docker 镜像到仓库
docker-push:
	@echo "$(GREEN)📤 Pushing Docker image...$(NC)"
	docker push $(BINARY_NAME):$(VERSION)
	docker push $(BINARY_NAME):latest
	@echo "$(GREEN)✅ Images pushed successfully!$(NC)"

## docker-clean: 清理 Docker 资源
docker-clean:
	@echo "$(RED)🧹 Cleaning Docker resources...$(NC)"
	-docker stop $(BINARY_NAME)-tui $(BINARY_NAME)-daemon 2>/dev/null || true
	-docker rm $(BINARY_NAME)-tui $(BINARY_NAME)-daemon 2>/dev/null || true
	-docker rmi $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest 2>/dev/null || true
	@echo "$(GREEN)✅ Docker cleanup complete!$(NC)"

# =============================================================================
# CI/CD 目标
# =============================================================================

## CI: 完整的 CI 流水线（lint + test + build + security）
ci: lint vet test build security
	@echo "$(GREEN)✅ CI pipeline completed successfully!$(NC)"

## CD: 完整的 CD 流水线（CI + release + docker-build）
cd: ci release docker-build
	@echo "$(GREEN)✅ CD pipeline completed successfully!$(NC)"

## pre-commit: Git pre-commit hook（格式化 + 检查）
pre-commit: fmt check vet lint test-short
	@echo "$(GREEN)✅ Pre-commit checks passed!$(NC)"

## post-commit: Git post-commit hook（可选的通知或部署触发）
post-commit:
	@echo "$(GREEN)📤 Post-commit actions...$(NC)"
	@# 可以在这里添加通知、CI 触发等操作
	@echo "$(GREEN)✅ Post-commit complete!$(NC)"

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
	@echo "  $(GREEN)build$(NC)           Cross-compile for macOS/Linux/Windows to dist/"
	@echo "  $(GREEN)build-current$(NC)   Compile only for current platform"
	@echo "  $(GREEN)build-debug$(NC)     Compile debug binary (with symbols)"
	@echo "  $(GREEN)setup-cross$(NC)     Install cross-compilation toolchains (brew)"
	@echo "  $(GREEN)install$(NC)         Install to system PATH (requires sudo)"
	@echo "  $(GREEN)uninstall$(NC)       Remove from system PATH"
	@echo ""
	@echo "$(YELLOW)▶️ Run Targets:$(NC)"
	@echo "  $(GREEN)run$(NC)             Start TUI (default mode)"
	@echo "  $(GREEN)run-daemon$(NC)      Start Daemon service"
	@echo "  $(GREEN)run-verbose$(NC)     Start TUI with debug logging"
	@echo "  $(GREEN)stop$(NC)             Stop running mindx daemon"
	@echo ""
	@echo "$(YELLOW)🛠️ Development Targets:$(NC)"
	@echo "  $(GREEN)dev$(NC)             Dev mode (hot-reload)"
	@echo "  $(GREEN)dev-tui$(NC)        Quick TUI run"
	@echo "  $(GREEN)dev-daemon$(NC)     Quick Daemon run"
	@echo "  $(GREEN)dev-watch$(NC)      File watcher auto-reload"
	@echo ""
	@echo "$(YELLOW)🧪 Test Targets:$(NC)"
	@echo "  $(GREEN)test$(NC)            Run unit tests with coverage"
	@echo "  $(GREEN)test-short$(NC)      Quick tests (skip integration)"
	@echo "  $(GREEN)test-integration$(NC) Integration tests only"
	@echo "  $(GREEN)test-race$(NC)       Race condition detection"
	@echo "  $(GREEN)bench$(NC)           Performance benchmarks"
	@echo "  $(GREEN)bench-compare$(NC)   Compare benchmark results"
	@echo ""
	@echo "$(YELLOW)✨ Quality Targets:$(NC)"
	@echo "  $(GREEN)lint$(NC)            Code quality check (golangci-lint)"
	@echo "  $(GREEN)lint-fix$(NC)        Auto-fix lint issues"
	@echo "  $(GREEN)vet$(NC)             Static analysis (go vet)"
	@echo "  $(GREEN)fmt$(NC)             Format code (gofmt)"
	@echo "  $(GREEN)check$(NC)           Verify formatting"
	@echo "  $(GREEN)imports$(NC)         Organize imports"
	@echo ""
	@echo "$(YELLOW)🔒 Security Targets:$(NC)"
	@echo "  $(GREEN)security$(NC)        Vulnerability scan (govulncheck)"
	@echo "  $(GREEN)audit$(NC)           Full dependency audit"
	@echo "  $(GREEN)vulnerability-check$(NC) CVE scanner"
	@echo ""
	@echo "$(YELLOW)📦 Dependency Targets:$(NC)"
	@echo "  $(GREEN)tidy$(NC)            Clean up module dependencies"
	@echo "  $(GREEN)deps$(NC)            Download all dependencies"
	@echo "  $(GREEN)deps-update$(NC)     Update to latest versions"
	@echo "  $(GREEN)vendor$(NC)          Create vendor directory"
	@echo ""
	@echo "$(YELLOW)📝 Documentation Targets:$(NC)"
	@echo "  $(GREEN)docs$(NC)            Generate API documentation"
	@echo "  $(GREEN)docs-serve$(NC)      Start local doc server"
	@echo "  $(GREEN)changelog$(NC)       Generate CHANGELOG.md"
	@echo ""
	@echo "$(YELLOW)🌍 Cross-Compile Targets:$(NC)"
	@echo "  $(GREEN)cross-build$(NC)     Build for all platforms"
	@echo "  $(GREEN)build-linux-amd64$(NC)   Linux x86_64"
	@echo "  $(GREEN)build-linux-arm64$(NC)   Linux ARM64"
	@echo "  $(GREEN)build-darwin-amd64$(NC)   macOS Intel"
	@echo "  $(GREEN)build-darwin-arm64$(NC)   macOS Apple Silicon"
	@echo "  $(GREEN)build-windows-amd64$(NC)  Windows x86_64"
	@echo ""
	@echo "$(YELLOW)🚀 Release Targets:$(NC)"
	@echo "  $(GREEN)release$(NC)          Create release packages"
	@echo "  $(GREEN)release-sign$(NC)     Generate checksums"
	@echo "  $(GREEN)release-notes$(NC)    Generate release notes"
	@echo ""
	@echo "$(YELLOW)🐳 Docker Targets:$(NC)"
	@echo "  $(GREEN)docker$(NC)            Docker Compose dev workflow (build + up)"
	@echo "  $(GREEN)docker-build$(NC)     Build Docker image"
	@echo "  $(GREEN)docker-run$(NC)       Run container (TUI)"
	@echo "  $(GREEN)docker-push$(NC)      Push image to registry"
	@echo "  $(GREEN)docker-clean$(NC)     Clean up Docker resources"
	@echo ""
	@echo "$(YELLOW)🔄 CI/CD Targets:$(NC)"
	@echo "  $(GREEN)ci$(NC)               Full CI pipeline"
	@echo "  $(GREEN)cd$(NC)               Full CD pipeline"
	@echo "  $(GREEN)pre-commit$(NC)       Git pre-commit hook"
	@echo "  $(GREEN)post-commit$(NC)      Git post-commit hook"
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
	@echo "  make test && make bench                   # Test and benchmark"
	@echo "  make fmt && make lint                     # Format and lint"
	@echo "  make ci                                   # Full CI pipeline"
	@echo "  make release && make release-notes         # Create release"
	@echo "  make cross-build                          # Multi-platform build"
	@echo "  make docker-build && make docker-run       # Docker workflow"
	@echo ""

# =============================================================================
# 清理目标
# =============================================================================

## clean: 清理所有构建产物和临时文件
clean:
	@echo "$(RED)🧹 Cleaning build artifacts...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -rf $(COVERAGE_DIR)
	rm -rf $(BENCHMARK_DIR)
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

# =============================================================================
# 内部目标（辅助功能）
# =============================================================================

pre-build:
	@echo "$(YELLOW)⏳ Pre-build checks...$(NC)"
	@test -n "$(GO)" || (echo "$(RED)❌ Error: Go not found$(NC)" && exit 1)
	@$(GO) version >/dev/null 2>&1 || (echo "$(RED)❌ Error: Go version check failed$(NC)" && exit 1)
	@echo "$(GREEN)✅ Pre-build checks passed!$(NC)"

post-build: build-current
	@echo "$(GREEN)📊 Post-build summary:$(NC)"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)
	@du -sh $(BUILD_DIR)/$(BINARY_NAME)

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
