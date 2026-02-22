# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MindX is a lightweight, self-evolving AI personal assistant with a bionic brain architecture. It's written in Go (backend) and React/TypeScript (frontend dashboard), designed to run primarily on local models via Ollama with optional cloud model fallback.

## Core Architecture

### Bionic Brain System (仿生大脑架构)

The system implements a dual-hemisphere brain model inspired by human cognition:

- **Left Brain (潜意识层 - Subconscious)**: Fast, automatic, low-power processing for simple interactions (weather queries, messages). Uses lightweight local models for quick responses.
- **Right Brain (主意识层 - Conscious)**: Deep, focused, high-quality processing for complex tasks (coding, decision-making). Uses more powerful models when needed.
- **Consciousness Manager**: Routes requests between left/right brain based on complexity, manages token budgets, and handles fallback logic.

Key files:
- `internal/usecase/brain/brain.go`: Main BionicBrain implementation
- `internal/usecase/brain/consciousness_manager.go`: Request routing logic
- `internal/usecase/brain/thinking.go`: Core thinking/inference engine
- `internal/core/brain.go`: Brain interface definitions

### Clean Architecture Layers

1. **cmd/**: Application entry point
2. **internal/adapters/**: External interfaces (CLI, HTTP, channels)
3. **internal/usecase/**: Business logic (brain, skills, memory, training, cron)
4. **internal/core/**: Domain interfaces and entities
5. **internal/infrastructure/**: Technical implementations (persistence, embedding, llama)
6. **internal/entity/**: Domain models
7. **pkg/**: Reusable utilities (logging, i18n, llama wrapper)

### Memory System

Long-term memory with automatic consolidation:
- `internal/usecase/memory/`: Memory management logic
- `internal/infrastructure/persistence/`: BadgerDB-based storage
- Automatic memory extraction from conversations
- Vector embeddings for semantic search

### Skills System

Extensible tool/skill framework compatible with OpenClaw ecosystem:
- `skills/`: Built-in skills (calculator, calendar, file_search, etc.)
- `internal/usecase/skills/`: Skill manager and execution
- Supports MCP (Model Context Protocol)
- Skills can be written in any language as CLI tools

### Training System

Self-evolution through fine-tuning on conversation data:
- `training/`: Python-based fine-tuning scripts
- `internal/usecase/training/`: Training data preparation
- Exports conversation data for model training
- Supports Ollama model format

## Common Development Commands

### Building and Running

```bash
# Build everything (frontend + backend)
make build

# Install to system
make install

# Development mode (hot reload for both frontend and backend)
make dev

# Run dashboard only
make run

# Run TUI chat interface
make run-tui

# Start kernel service
make run-kernel
```

### Testing

```bash
# Run all tests (uses .test workspace to avoid polluting data)
make test

# Test specific package
MINDX_WORKSPACE=$(PWD)/.test go test ./internal/usecase/brain/...
```

### Code Quality

```bash
# Format code
make fmt

# Lint code
make lint

# Check environment for issues
make doctor
```

### Building Releases

```bash
# Build for all platforms (binaries only)
make build-all

# Build Linux release packages (AMD64 + ARM64)
make build-linux-release

# Build Windows release packages
make build-windows-release

# Build all release packages
make build-all-releases
```

### Model and Skill Management

```bash
# Test model compatibility
make run-model-test

# List all skills
make run-skill-list

# Run training once
make run-train
```

## Configuration

Configuration files are located in `$MINDX_WORKSPACE/config/` (default: `~/.mindx/config/`):

- `models.json`: Model configurations (left brain, right brain, capabilities)
- `capabilities.json`: Capability-specific model bindings
- `channels.json`: Communication channel configurations (WeChat, DingTalk, Telegram, etc.)
- `general.json`: General settings (persona, token budget, etc.)

## Environment Variables

- `MINDX_WORKSPACE`: Working directory (default: `~/.mindx`)
- `MINDX_SKILLS_DIR`: Skills directory (default: `~/.mindx/skills`)
- `BOT_DEV_MODE`: Enable development mode

## Key Technical Details

### Model Integration

- Primary: Ollama for local models (via `pkg/llama/`)
- Fallback: OpenAI-compatible APIs (via `github.com/sashabaranov/go-openai`)
- Model manager: `internal/config/model.go` handles model selection and routing

### Token Budget Management

- `internal/usecase/brain/token_budget.go`: Tracks and limits token usage
- Configurable budgets per time window (hourly, daily, monthly)
- Automatic fallback to cheaper models when budget exceeded

### Tool Calling

- `internal/usecase/brain/tool_caller.go`: Executes skills based on LLM decisions
- Supports function calling with structured JSON arguments
- Automatic retry and error handling

### Streaming Responses

- `internal/usecase/brain/thinking_stream.go`: Real-time streaming of LLM responses
- WebSocket support for live updates to dashboard/TUI
- Event types: Start, Progress, Chunk, ToolCall, ToolResult, Complete, Error

### Cron/Scheduling

- `internal/usecase/cron/`: Scheduled task management
- LLM can create scheduled tasks from natural language
- Persistent storage of cron jobs

## Frontend (Dashboard)

- Location: `dashboard/`
- Tech stack: React 18, TypeScript, Vite, TailwindCSS, TDesign React
- Key features: Chat interface, model management, skill management, training interface
- Build: `cd dashboard && npm run build`
- Dev: `cd dashboard && npm run dev`

## Important Notes

- Always use `MINDX_WORKSPACE=$(PWD)/.test` when running tests to avoid polluting production data
- The system requires Ollama to be running for local model inference
- Skills are discovered automatically from `$MINDX_WORKSPACE/skills/` and `$MINDX_INSTALL_PATH/skills/`
- Memory consolidation runs automatically in the background
- Training data is exported to `$MINDX_WORKSPACE/data/training/`

## CLI Command Reference

See `internal/adapters/cli/README.md` for complete CLI documentation.

Main commands:
- `mindx version`: Show version info
- `mindx dashboard`: Open web console
- `mindx tui`: Start terminal chat interface
- `mindx kernel start|stop|restart|status`: Service control
- `mindx model test`: Test model compatibility
- `mindx skill list|install|uninstall`: Skill management
- `mindx train run|export`: Training operations
