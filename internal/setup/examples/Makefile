# =============================================================================
# MindX Setup 向导组件示例 - Makefile
# =============================================================================
# 每个 target 对应一个独立步骤组件，方便手动查看实际渲染效果
#
# 常用命令:
#   make help               # 列出所有可用 target（默认）
#   make <step-name>        # 运行指定步骤组件示例
#   make all                # 编译验证所有示例（不运行）
#
# =============================================================================

.PHONY: help all modelselect apikeyinput daemoncheck pythoncheck memoryconfig pathsetup

ROOT := ../../..
SRC  := internal/setup/examples

help:
	@echo "MindX Setup 向导组件示例"
	@echo ""
	@echo "使用方式:"
	@echo "  make <target>"
	@echo ""
	@echo "Available targets (向导步骤):"
	@echo "  modelselect   — 步骤0: 模型选择列表（↑↓ 选择 / Enter 确认 / S 跳过）"
	@echo "  apikeyinput   — 步骤1: API Key 密码输入（密码模式 / Esc 返回）"
	@echo "  daemoncheck   — 步骤2: Daemon 后台服务检查（← → 切换 Yes/No）"
	@echo "  pythoncheck   — 步骤3: Python 环境检测（按 1/2/3 切换场景）"
	@echo "  memoryconfig  — 步骤4: 记忆体 Embedder 配置（含下载进度）"
	@echo "  pathsetup     — 步骤5: 系统 PATH 配置（Windows 专用）"
	@echo ""
	@echo "Other:"
	@echo "  all           — 编译验证所有示例（不运行）"
	@echo ""

all:
	@echo "编译验证所有示例..."
	cd $(ROOT) && go build ./$(SRC)/...

define RUN_EXAMPLE
	cd $(ROOT) && go run ./$(SRC)/$(1)
endef

modelselect:   ; $(call RUN_EXAMPLE,modelselect)
apikeyinput:   ; $(call RUN_EXAMPLE,apikeyinput)
daemoncheck:   ; $(call RUN_EXAMPLE,daemoncheck)
pythoncheck:   ; $(call RUN_EXAMPLE,pythoncheck)
memoryconfig:  ; $(call RUN_EXAMPLE,memoryconfig)
pathsetup:     ; $(call RUN_EXAMPLE,pathsetup)
