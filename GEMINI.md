# MindX

## Project Overview

MindX is a lightweight, thinking, and self-evolving AI digital twin/personal assistant. It utilizes a "bionic brain architecture" with subconscious and conscious layers to optimize the use of local Large Language Models (LLMs) via Ollama, reducing the need for cloud-based inference and saving on token costs.

Key features include:
- **Bionic Brain Architecture:** Layered thinking (fast subconscious vs. deep conscious).
- **Long-term Memory:** Automatically extracts and manages memories from conversations.
- **Skill Ecosystem:** Compatible with OpenClaw skills and the MCP protocol.
- **Autonomous Evolution:** Supports local fine-tuning of models to adapt to user preferences.
- **Multi-channel Support:** Integrates with DingTalk, WeChat, QQ, Feishu, WhatsApp, Telegram, Facebook, etc.

The project is structured as a monorepo containing a Go-based backend and a React/TypeScript/Vite-based frontend dashboard.

## Key Directories and Files

- `cmd/main.go`: The main entry point for the Go application.
- `internal/`: Core application logic following clean architecture principles (`adapters`, `core`, `entity`, `infrastructure`, `usecase`).
- `dashboard/`: The frontend web interface built with React, TypeScript, Vite, and Tailwind CSS.
- `scripts/`: Various shell and Python scripts for building, installing, and development.
- `skills/`: A collection of executable skills/tools available to the AI (e.g., web search, calculator, weather).
- `config/`: Configuration files and templates for server, models, channels, and capabilities.
- `training/`: Scripts and Python tools for fine-tuning local models (LoRA).
- `pkg/`: Reusable Go packages (errors, i18n, llama, logging).
- `Makefile`: The central entry point for all build, development, and operational commands.

## Building and Running

The project relies heavily on `make` for its lifecycle commands.

### Prerequisites
- **Go:** 1.21 or later.
- **Node.js:** 18 or later (with `npm`).
- **Ollama:** Required for local model execution (`brew install ollama` or `curl -fsSL https://ollama.com/install.sh | sh`, then `ollama serve`).

### Common Commands

- **Start Development Mode:** `make dev`
  - Starts both the Go backend and the Vite frontend with hot-reloading. The frontend will typically be accessible at `http://localhost:5173`.
- **Build Project:** `make build`
  - Compiles the Go binary and builds the React frontend for production.
- **Install:** `make install`
  - Builds the project and installs the `mindx` CLI to your system.
- **Run Tests:** `make test`
  - Executes all Go unit tests within an isolated `.test` workspace.
- **Code Quality:**
  - Format code: `make fmt`
  - Lint code: `make lint`
- **Dependency Management:** `make deps`
  - Runs `go mod tidy` and `npm update` in the dashboard.
- **Run Interfaces:**
  - Web Dashboard: `make run-dashboard` (or `mindx dashboard`)
  - Terminal UI: `make run-tui` (or `mindx tui`)
  - Backend Kernel: `make run-kernel` (or `mindx kernel run`)

## Development Conventions

- **Backend Architecture:** The Go code heavily utilizes a layered, domain-driven/clean architecture pattern. Changes to business logic should generally flow from `entity` -> `usecase` -> `core` -> `adapters`.
- **Frontend Stack:** The dashboard is built using standard modern React practices (Hooks, Functional Components) with TypeScript for type safety and Tailwind CSS for styling.
- **Cross-Platform:** The build system supports cross-compilation for macOS (Intel/Apple Silicon Universal Binaries), Linux, and Windows.