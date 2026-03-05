# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MindX is a lightweight, self-evolving AI personal assistant with a bionic brain architecture. Written in Go (backend) and React/TypeScript (frontend dashboard). Runs primarily on local models via Ollama with optional cloud model fallback. Current version is in the `VERSION` file (v1.0.4).

## Core Architecture

### Bionic Brain System (仿生大脑架构)

Dual-hemisphere brain model inspired by human cognition:

- **Left Brain (潜意识层)**: Fast, automatic processing for simple interactions. Uses lightweight local models.
- **Right Brain (主意识层)**: Deep, focused processing for complex tasks. Uses more powerful models.
- **Consciousness Manager**: Routes requests between left/right brain based on complexity, manages token budgets, handles fallback.

Key files:
- `internal/usecase/brain/brain.go` — Main BionicBrain implementation
- `internal/usecase/brain/consciousness_manager.go` — Request routing logic
- `internal/usecase/brain/thinking.go` — Core thinking/inference engine
- `internal/core/brain.go` — Brain interface definitions (Thinking, Brain, Assistant interfaces)

### Clean Architecture Layers

```
cmd/main.go                          — Single entry point
internal/adapters/cli/               — CLI commands (Cobra)
internal/adapters/http/              — HTTP/WebSocket handlers (Gin)
internal/adapters/channels/          — Messaging channel gateway (WeChat, DingTalk, Telegram, etc.)
internal/usecase/brain/              — Brain logic (thinking, tool calling, streaming, token budget)
internal/usecase/memory/             — Memory management (extraction, consolidation, search)
internal/usecase/skills/             — Skill manager and execution
internal/usecase/session/            — Session/conversation management
internal/usecase/capability/         — Capability-model binding
internal/usecase/cron/               — Scheduled task management
internal/usecase/training/           — Training data preparation
internal/usecase/embedding/          — Embedding service
internal/core/                       — Domain interfaces (Brain, Thinking, Memory, SkillManager, etc.)
internal/entity/                     — Domain models (capability, channel, session, skill, tool, etc.)
internal/infrastructure/bootstrap/   — App initialization (Startup/Shutdown lifecycle)
internal/infrastructure/persistence/ — BadgerDB storage
internal/infrastructure/embedding/   — Ollama embedding provider
internal/infrastructure/llama/       — LLM client wrapper
internal/config/                     — Configuration management (Viper-based)
pkg/llama/                           — Reusable Ollama/OpenAI client wrapper
pkg/logging/                         — Zap-based structured logging
pkg/i18n/                            — Internationalization
pkg/circuitbreaker/                  — Circuit breaker pattern
pkg/retry/                           — Retry utilities
```

### Bootstrap & Lifecycle

`internal/infrastructure/bootstrap/app.go` contains `Startup()` which initializes the entire application in order: workspace → config → logging → models → embedding → persistence → memory → skills → capabilities → brain → assistant → sessions → cron → channels → HTTP server. `Shutdown()` tears down in reverse. The `App` struct holds all top-level components.

### Key Interfaces (internal/core/)

- `Thinking` — Think, ThinkWithTools, ReturnFuncResult, stream variants
- `Brain` — GetThinking, GetTokenBudget, consciousness routing
- `Assistant` — Ask, Summarize, persona management, access to brain/memory/skills
- `Memory` — Store, Search, Consolidate
- `SkillManager` — Discover, Execute, Install/Uninstall skills
- `SessionMgr` — Conversation session lifecycle
- `Channel` — Messaging channel abstraction (Start, Stop, Send, Receive)

### Skills System

Skills live in `skills/` as standalone CLI tools (any language). Each skill has a `skill.json` manifest. Built-in skills include: calculator, calendar, cron, deep_search, file_search, github, mail, notes, reminders, screenshot, terminal, weather, web_search, and more. Skills are discovered from `$MINDX_WORKSPACE/skills/` and `$MINDX_INSTALL_PATH/skills/`. MCP (Model Context Protocol) is supported via `github.com/modelcontextprotocol/go-sdk`.

### Streaming Responses

`internal/usecase/brain/thinking_stream.go` handles real-time streaming. Event types: Start, Progress, Chunk, ToolCall, ToolResult, Complete, Error. WebSocket support for live updates.

## Development Commands

### Build & Run

```bash
make build              # Build frontend + backend
make install            # Build and install to system
make dev                # Hot reload dev mode (frontend + backend)
make run                # Start dashboard
make run-tui            # Terminal chat interface
make run-kernel         # Start kernel service
make clean              # Clean build artifacts (bin/, dist/, dashboard/dist/)
```

### Testing

```bash
make test               # Run all tests (auto-creates .test workspace)

# Test a specific package:
MINDX_WORKSPACE=$(PWD)/.test go test ./internal/usecase/brain/...

# Skip integration tests:
MINDX_WORKSPACE=$(PWD)/.test go test -short ./...

# Frontend tests:
cd dashboard && npm run test
```

Tests use `MINDX_WORKSPACE=$(PWD)/.test` to isolate from production data. Config files are copied from `config/` into `.test/config/`. Integration tests in `internal/tests/` use `bootstrap.Startup()` and can be skipped with `-short`. The project uses `testify` for assertions.

### Code Quality

```bash
make fmt                # Format Go code
make lint               # Run golangci-lint (govet, errcheck, staticcheck, unused)
make doctor             # Check environment for issues
```

### CI

GitHub Actions (`.github/workflows/ci.yml`) runs on push/PR to `main`:
- Backend: `go vet ./...` then `go test ./...` with test workspace
- Frontend: `npm ci` → `npm run lint` → `npx tsc --noEmit` → `npm run test`

### Release Builds

```bash
make build-all              # All platforms (binaries only)
make build-linux-release    # Linux packages (AMD64 + ARM64)
make build-windows-release  # Windows packages
make build-all-releases     # All release packages
```

Build scripts live in `scripts/` (build.sh, install.sh, dev-start.sh, doctor.sh, etc.).

## Configuration

Config files in `config/` (copied to `$MINDX_WORKSPACE/config/` at runtime, YAML format):

- `models.yml` — Model configurations (left brain, right brain, embedding model, base URLs)
- `capabilities.yml` — Capability-specific model bindings
- `channels.yml` — Communication channel configs (WeChat, DingTalk, Telegram, etc.)
- `server.yml` — Server settings (port, persona, token budget)
- `mcp_servers.json.template` — MCP server configuration template

Config is managed via Viper (`internal/config/`). `config.InitVippers()` loads all config files.

## Environment Variables

- `MINDX_WORKSPACE` — Working directory (default: `~/.mindx`)
- `MINDX_SKILLS_DIR` — Skills directory (default: `~/.mindx/skills`)
- `BOT_DEV_MODE` — Enable development mode

## Frontend (Dashboard)

- Location: `dashboard/`
- Stack: React 18, TypeScript 5.6, Vite 5, TailwindCSS, TDesign React
- Features: Chat interface, model management, skill management, markdown rendering (with math/mermaid support)
- Dev: `cd dashboard && npm run dev` (port 5173)
- Build: `cd dashboard && npm run build` (output embedded in Go binary)
- Lint: `cd dashboard && npm run lint`
- Test: `cd dashboard && npm run test` (vitest)

## Key Dependencies

- **Go 1.25.1** — Web: Gin, CLI: Cobra/Viper, DB: BadgerDB, LLM: go-openai, MCP: go-sdk, TUI: bubbletea/lipgloss, Logging: Zap, Testing: testify
- **Node 20+** — React 18, Vite 5, TDesign React, vitest

## Important Notes

- Always use `MINDX_WORKSPACE=$(PWD)/.test` when running tests
- Ollama must be running for local model inference (default: `http://localhost:11434`)
- Default embedding model: `qllama/bge-small-zh-v1.5:latest`
- The codebase uses Chinese comments extensively — this is intentional
- Memory consolidation and training run as background/cron tasks
- The `next` branch is used for active development; `main` is the stable branch
