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

.PHONY: build build-current setup-cross clear install run run-daemon test bench lint clean docs tidy help \
        dev dev-tui dev-daemon uninstall format check vet \
        release release-all cross-build docker-build docker-push \
        generate proto swagger ci cd security audit deps-update \
        pre-commit post-commit version info changelog

# =============================================================================
# 配置变量
# =============================================================================

# 项目信息
BINARY_NAME    ?= mindx
PROJECT_NAME   ?= mindx
VERSION        ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v2.1.0")
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

# 构建标志
LDFLAGS        ?= -s -w \
                 -X main.version=$(VERSION) \
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
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
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
	@echo "$(GREEN)➡ Building $(BINARY_NAME) v$(VERSION) for $(shell $(GO) env GOOS)/$(shell $(GO) env GOARCH)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
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

## install: 编译并安装到系统路径（需要 sudo 权限）
install: build-current
	@echo "$(GREEN)➡ Installing $(BINARY_NAME)...$(NC)"
	@if command -v sudo >/dev/null 2>&1; then \
		sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME) && \
		echo "$(GREEN)✅ Installed to /usr/local/bin/$(BINARY_NAME)$(NC)"; \
	else \
		cp $(BUILD_DIR)/$(BINARY_NAME) $$GOPATH/bin/$(BINARY_NAME) && \
		echo "$(GREEN)✅ Installed to $$GOPATH/bin/$(BINARY_NAME)$(NC)"; \
	fi
	@echo "$(BLUE)🎉 Installation complete! Run '$(BINARY_NAME)' to start.$(NC)"

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
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "$(GREEN)✅ darwin/amd64 done$(NC)"

## build-darwin-arm64: macOS Apple Silicon (M1/M2)
build-darwin-arm64:
	@echo "$(GREEN)➡ Building for darwin/arm64 (Apple Silicon)...$(NC)"
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "$(GREEN)✅ darwin/arm64 done$(NC)"

## build-windows-amd64: Windows x86_64
build-windows-amd64:
	@echo "$(GREEN)➡ Building for windows/amd64...$(NC)"
	@mkdir -p $(DIST_DIR)
	@if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe . && \
		echo "$(GREEN)✅ windows/amd64 done$(NC)"; \
	else \
		echo "$(YELLOW)⚠  windows/amd64 skipped — install: brew install mingw-w64$(NC)"; \
	fi

# =============================================================================
# 发布目标
# =============================================================================

## release: 创建发布包（当前版本）
release: clean cross-build
	@echo "$(GREEN)📦 Creating release archives...$(NC)"
	@mkdir -p releases
	@# Linux AMD64
	tar -czf releases/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(DIST_DIR) $(BINARY_NAME)-linux-amd64
	@# Linux ARM64
	tar -czf releases/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz -C $(DIST_DIR) $(BINARY_NAME)-linux-arm64
	@# Darwin (separate amd64 + arm64 tarballs)
	tar -czf releases/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz -C $(DIST_DIR) $(BINARY_NAME)-darwin-amd64
	tar -czf releases/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz -C $(DIST_DIR) $(BINARY_NAME)-darwin-arm64
	@# Windows
	zip -j releases/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe
	@echo "$(GREEN)✅ Release packages created in releases/$(NC)"
	@ls -lh releases/

## release-sign: 为发布包创建校验和（用于验证完整性）
release-sign:
	@echo "$(GREEN)🔐 Generating checksums...$(NC)"
	@cd releases && shasum -a 256 *.tar.gz *.zip > checksums.txt
	@echo "$(GREEN)✅ Checksums created: releases/checksums.txt$(NC)"

## release-notes: 生成发布说明
release-notes:
	@echo "$(GREEN)📝 Generating release notes...$(NC)"
	@printf '%s\n' "# $(BINARY_NAME) $(VERSION) Release Notes" "" > RELEASE_NOTES.md
	@printf '%s\n' "## 📦 Downloads" "" >> RELEASE_NOTES.md
	@printf '| %s | %s | %s |\n' "Platform" "File" "SHA256" >> RELEASE_NOTES.md
	@printf '|%s|%s|%s|\n' "----------" "------" "------" >> RELEASE_NOTES.md
	@for f in releases/*.tar.gz releases/*.zip; do \
		platform=$$(basename "$$f" | sed 's/\.tar\.gz$$//;s/\.zip$$//;s/^[^-]*-[^-]*-//'); \
		sha=$$(shasum -a 256 "$$f" | cut -d' ' -f1); \
		printf '| %s | %s | %s |\n' "$$platform" "$$(basename "$$f")" "$$sha" >> RELEASE_NOTES.md; \
	done
	@printf '%s\n' "" "## ✨ Changes" "" >> RELEASE_NOTES.md
	@git log --oneline --no-merges "$(shell git describe --tags --abbrev=0 2>/dev/null)..HEAD" 2>/dev/null >> RELEASE_NOTES.md || echo "- Initial release" >> RELEASE_NOTES.md
	@echo "$(GREEN)✅ Release notes created: RELEASE_NOTES.md$(NC)"

# =============================================================================
# Docker 目标
# =============================================================================

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
