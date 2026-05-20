# =============================================================================
# MindX Client 组件示例 - Makefile
# =============================================================================
# 每个 target 对应一个独立组件示例，方便手动查看实际渲染效果
#
# 常用命令:
#   make help               # 列出所有可用 target（默认）
#   make <component-name>   # 运行指定组件示例
#   make all                # 编译验证所有示例（不运行）
#
# =============================================================================

.PHONY: help all welcome question thinking action output conversation conv \
        input notify statusbar choices error reasoning

ROOT := ../../..
SRC  := internal/client/examples

help:
	@echo "MindX Client 组件示例"
	@echo ""
	@echo "使用方式:"
	@echo "  make <target>"
	@echo ""
	@echo "Available targets:"
	@echo "  welcome       — WelcomePanel 欢迎页（渐变标题 + 元信息）"
	@echo "  question      — Question 用户问题显示"
	@echo "  thinking      — Thinking 思考日志（按 t 折叠 / 按 b 触发 blink）"
	@echo "  action        — Action 工具执行（按 1/2 切换状态）"
	@echo "  output        — Output Markdown 结果渲染"
	@echo "  conversation  — Conversation 单次完整对话"
	@echo "  conv          — ConversationList 整体容器（按 c 切换对话）"
	@echo "  input         — InputArea 输入框（/ 命令 / @ 提示）"
	@echo "  notify        — NotificationBar 通知栏（i/s/e/w 触发通知）"
	@echo "  statusbar     — StatusBar 状态栏（按 1/2 切换连接状态）"
	@echo "  choices       — ChoicesPanel 选择面板（按 s 弹出选项）"
	@echo "  error         — ErrorMsg 错误提示（按 t/a/o 触发不同阶段错误）"
	@echo "  reasoning     — Reasoning 推理状态（按 1-5 切换不同场景）"
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

welcome:      ; $(call RUN_EXAMPLE,welcome)
question:     ; $(call RUN_EXAMPLE,question)
thinking:     ; $(call RUN_EXAMPLE,thinking)
action:       ; $(call RUN_EXAMPLE,action)
output:       ; $(call RUN_EXAMPLE,output)
conversation: ; $(call RUN_EXAMPLE,conversation)
conv:         ; $(call RUN_EXAMPLE,conv)
input:        ; $(call RUN_EXAMPLE,input)
notify:       ; $(call RUN_EXAMPLE,notify)
statusbar:    ; $(call RUN_EXAMPLE,statusbar)
choices:      ; $(call RUN_EXAMPLE,choices)
error:        ; $(call RUN_EXAMPLE,error)
reasoning:    ; $(call RUN_EXAMPLE,reasoning)
