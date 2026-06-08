# Contributing to MindX

Thank you for your interest in contributing to MindX! This document provides guidelines and instructions for contributing to the project.

---

## Table of Contents

- [Contributing to MindX](#contributing-to-mindx)
  - [Table of Contents](#table-of-contents)
  - [Code of Conduct](#code-of-conduct)
  - [Getting Started](#getting-started)
  - [Development Setup](#development-setup)
    - [Prerequisites](#prerequisites)
    - [Build \& Run](#build--run)
    - [First Run](#first-run)
  - [Project Structure](#project-structure)
  - [Workflow](#workflow)
  - [Coding Standards](#coding-standards)
    - [Go](#go)
    - [Documentation](#documentation)
    - [Skills (SKILL.md)](#skills-skillmd)
  - [Commit Message Guidelines](#commit-message-guidelines)
  - [Pull Request Process](#pull-request-process)
  - [Reporting Bugs](#reporting-bugs)
  - [Suggesting Features](#suggesting-features)
  - [Recognition](#recognition)

---

## Code of Conduct

This project adheres to a code of conduct that all contributors are expected to follow. Please be respectful and constructive in all interactions within this project.

---

## Getting Started

1. **Fork the repository** and clone your fork locally:

```bash
git clone https://github.com/<your-username>/mindx.git
cd mindx
```

2. **Add the upstream remote** to keep your fork up-to-date:

```bash
git remote add upstream https://github.com/DotNetAge/mindx.git
```

3. **Create a branch** for your changes:

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

---

## Development Setup

### Prerequisites

- **Go 1.21+** with CGO enabled (`CGO_ENABLED=1`)
- **Make** for running build commands
- **Docker** (optional, for containerized development)

### Build & Run

```bash
make build        # Build the project
make run          # Run TUI interface
make run-daemon   # Run Daemon in background
make test         # Run tests
make fmt          # Format code
make lint         # Lint code
```

### First Run

After building, run `./dist/mindx` (or `make run`) to start the interactive setup wizard, which will guide you through API key configuration, model selection, and initial setup.

---

## Project Structure

```
mindx/
├── .github/workflows/    # CI/CD pipelines (ci, release, security)
├── assets/images/        # Static images (architecture diagrams, screenshots)
├── cmd/                  # CLI command definitions (root, agent, doctor, logs, etc.)
├── config/               # Default configuration files (channels, server)
├── internal/
│   ├── client/           # TUI client (Bubble Tea components, renderers, themes, styles)
│   │   ├── component/    # UI components (conversation, input, sidebar, statusbar, dialog, etc.)
│   │   ├── data/         # Client-side data models
│   │   ├── examples/     # Component usage examples
│   │   ├── msg/          # Message type definitions
│   │   ├── render/       # Markdown/table/todo rendering
│   │   └── style/        # Theme and style definitions
│   ├── commands/         # Slash commands catalog and scheduler
│   ├── core/             # Core application logic (app, config, providers, credentials, workspace)
│   ├── setup/            # Interactive setup wizard (API key, model, path, daemon, Python checks)
│   └── svc/              # Daemon service (RPC handlers, event dispatcher, web server, wiring)
├── pkg/
│   ├── logging/          # Structured logger (zap + lumberjack)
│   ├── memory/           # Memory & RAG services (project indexer, file watching, mindxignore)
│   ├── scheduler/        # Cron scheduler for periodic tasks
│   └── session/          # Session persistence (file store, RAG adapter, summarizer, token estimator)
├── runtime/
│   ├── agents/           # Agent definition files (.md)
│   ├── settings/         # Runtime settings (models, providers)
│   └── skills/           # Built-in skills (SKILL.md with scripts, templates, references)
├── Dockerfile            # Multi-stage Docker build
├── docker-compose.yml    # Docker Compose with health checks and volume mounts
├── embed.go              # Go embed for runtime assets
├── go.mod / go.sum       # Go module definition
├── main.go               # Application entry point
└── Makefile              # Build automation
```

---

## Workflow

1. **Sync your fork** before starting work:

```bash
git fetch upstream
git rebase upstream/main
```

2. **Make your changes** following the coding standards below.

3. **Test your changes** thoroughly:

```bash
make test
make lint
```

4. **Commit your changes** with clear messages (see guidelines below).

5. **Push to your fork** and open a Pull Request.

---

## Coding Standards

### Go

- Follow [Effective Go](https://go.dev/doc/effective_go) and standard Go conventions.
- Run `make fmt` before committing to ensure consistent formatting.
- Run `make lint` to catch potential issues.
- Write tests for new functionality — aim for meaningful coverage of critical paths.
- Keep functions focused and concise; prefer composition over deep nesting.

### Documentation

- Public exported types and functions must have doc comments.
- Use complete sentences ending with punctuation.
- Keep comments focused on *why*, not *what* — the code shows *what*.

### Skills (SKILL.md)

When creating or modifying skills:

- Each skill lives in its own directory under `skills/` with a `SKILL.md` file.
- Include YAML frontmatter with metadata (name, description, triggers).
- Skills support hot-reload at runtime — no rebuild required for skill changes.

---

## Commit Message Guidelines

We follow conventional commit format:

```
<type>(<scope>): <subject>

<body>
```

**Types:**

| Type       | Description                                             |
| ---------- | ------------------------------------------------------- |
| `feat`     | New feature                                             |
| `fix`      | Bug fix                                                 |
| `docs`     | Documentation only                                      |
| `style`    | Formatting, no code change                              |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `perf`     | Performance improvement                                 |
| `test`     | Adding or updating tests                                |
| `chore`    | Build process, tooling, dependencies                    |

**Examples:**

```
feat(agent): add concurrent sub-agent execution mode

fix(memory): resolve race condition in memory index writer

docs(readme): update installation instructions for Linux

refactor(context): simplify session compression pipeline
```

---

## Pull Request Process

1. **Ensure your PR targets `main` branch** from your feature branch.
2. **Fill out the PR template** describing:
   - What problem does this PR solve?
   - How did you solve it?
   - Are there breaking changes? (if yes, describe migration steps)
   - Screenshots / demos (for UI changes)
3. **Ensure CI passes** — check GitHub Actions status.
4. **Keep PRs focused** — one logical change per PR makes review easier.
5. **Link related issues** using keywords like `Fixes #123` or `Closes #456`.
6. **Respond to review feedback** promptly and iteratively.

Once your PR is reviewed and approved by a maintainer, it will be merged. Contributors who submit quality PRs will be added to the core contributor list.

---

## Reporting Bugs

Before submitting a bug report:

1. **Search existing issues** to avoid duplicates.
2. **Collect relevant information**:
   - MindX version (`mindx --version`)
   - Operating system and version
   - Steps to reproduce the issue
   - Expected vs actual behavior
   - Relevant logs (`mindx logs` output)

Open an issue with the `bug` label and include all collected information.

---

## Suggesting Features

We welcome feature suggestions! When proposing a new feature:

1. **Open an issue** with the `enhancement` label.
2. **Describe the use case** clearly — what problem does this solve?
3. **Consider the scope** — is this a core platform feature or a skill-level addition?
4. **Discuss implementation** if you have ideas on how it could work.

Feature requests that align with MindX's design philosophy ("skills over tools") and provide clear user value are prioritized.

---

## Recognition

All contributors — whether code, documentation, bug reports, or feature suggestions — are valued and appreciated. Regular contributors will be recognized in the project's contributor roster.

Thank you for helping make MindX better!
