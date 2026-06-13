---
name: docker-expert
description: >
  This skill should be used when the user asks to "create a Dockerfile", "optimize Docker image",
  "multi-stage build", "container security", "Docker Compose", "docker-compose.yml",
  "reduce image size", "Docker best practices", "containerize this app", "Docker health check",
  "dockerignore", "Docker networking", "Docker volume", "Docker secrets", "container orchestration",
  "Docker build optimization", "non-root container", "Docker layer caching", or any request
  involving Docker containerization, image building, container security hardening, or
  Docker Compose service orchestration. Also use when reviewing Docker configurations,
  diagnosing build failures, troubleshooting container networking, or setting up development
  containers with hot reload. Provides comprehensive Docker expertise including multi-stage
  build optimization, security hardening, image size reduction, and production-ready patterns.
allowed-tools: bash read_file write_file
metadata:
  name_zh: Docker 专家
  name_zh-tw: Docker 專家
  description_zh: Docker 容器化优化、多阶段构建、安全加固与生产级部署配置
  description_zh-tw: Docker 容器化優化、多階段建置、安全加固與生產級部署配置
---

# Docker Expert Skill

Transform Docker configurations from functional to production-grade with focus on optimization, security, and maintainability. Use this when working with Dockerfiles, Docker Compose configurations, container builds, or any containerization task — from initial setup to production hardening.

## When to Use This Skill

- The user wants to create or improve a Dockerfile for any language or framework
- Someone asks to optimize an existing Docker image (size, build speed, caching)
- A project needs to be containerized from scratch or migrated to containers
- The user wants to set up Docker Compose for multi-service applications
- Security scanning reveals vulnerabilities in a container image
- Builds are slow and someone asks to speed up Docker layer caching
- A container needs security hardening (non-root user, secrets management, capabilities)
- The user asks to review a Docker configuration for production readiness
- Development workflow needs container setup with hot reload and debugging
- Someone encounters Docker networking issues between services
- The user asks about multi-architecture builds or cross-platform containerization

## What This Skill Does

1. **Analyzes existing Docker setups** — detects patterns, identifies anti-patterns, assesses production readiness
2. **Creates production-grade Dockerfiles** — multi-stage builds, layer optimization, security hardening
3. **Designs Docker Compose configurations** — service orchestration, networking, health checks, secrets
4. **Optimizes image size and build speed** — from bloated images to lean, cached builds
5. **Hardens container security** — non-root users, minimal attack surface, secrets management
6. **Diagnoses container issues** — build failures, networking problems, resource constraints

## How to Use

```
Create a Dockerfile for this Go API server
```

```
My Docker build takes 10 minutes. Can you optimize it and speed up the caching?
```

```
Review our docker-compose.yml for production readiness — we're deploying next week
```

```
Containerize this Node.js app with multi-stage builds for production
```

```
Our container security scan found 15 vulnerabilities. Help me harden the image.
```

```
Set up a development Docker Compose environment with hot reload for this React + Express app
```

## Workflow Overview

```
User describes a Docker task
        │
        ▼
  Phase 1: Environment Detection — what Docker version, what project structure, what exists
        │
        ▼
  Phase 2: Problem Analysis — categorize: build, security, networking, orchestration, optimization
        │
        ▼
  Phase 3: Solution Design — apply best-practice patterns matching the user's stack
        │
        ▼
  Phase 4: Implementation — write or modify Dockerfiles, compose files, .dockerignore
        │
        ▼
  Phase 5: Validation — build test, security scan, runtime verification
```

---

## Instructions

### Phase 1: Environment Detection

**Trigger:** Any Docker-related request. Always detect the environment first before making recommendations.

#### Step 1.1 - Check Docker Availability

```bash
docker --version 2>/dev/null || echo "Docker not installed"
docker info --format '{{.ServerVersion}}' 2>/dev/null || echo "Docker daemon not running"
```

| Condition              | Action                                                                 |
| ---------------------- | ---------------------------------------------------------------------- |
| Docker not installed   | Guide user to install Docker Desktop or Docker Engine before proceeding |
| Daemon not running     | Ask user to start Docker first                                         |
| Docker ready           | Proceed to Step 1.2                                                    |

#### Step 1.2 - Scan Project Structure

Find all Docker-related files in the project:

```bash
find . -name "Dockerfile*" -type f | head -10
find . -name "*compose*.yml" -o -name "*compose*.yaml" -type f | head -5
find . -name ".dockerignore" -type f | head -3
```

#### Step 1.3 - Assess Existing State

If Dockerfiles exist, examine them for patterns:

| What to Check                              | Why It Matters                                  |
| ------------------------------------------ | ----------------------------------------------- |
| Base image choice (Alpine, slim, distroless) | Affects size and security surface              |
| Multi-stage builds present?                | Key optimization opportunity if missing          |
| Layer ordering (deps before source?)       | Main cause of slow builds / cache invalidation   |
| USER directive present?                    | Security baseline — should never run as root     |
| HEALTHCHECK defined?                       | Needed for orchestration and production          |
| EXPOSE vs actual ports                     | Must match application listening port            |

#### Step 1.4 - Check Running State (if applicable)

```bash
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | head -10
docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}" | head -10
```

### Phase 2: Problem Analysis

**Trigger:** After environment detection. Categorize the user's request to apply the right solution approach.

#### Step 2.1 - Categorize the Request

| Category            | Typical Trigger Phrases                                        | Solution Focus                   |
| ------------------- | -------------------------------------------------------------- | -------------------------------- |
| **New Dockerfile**  | "create a Dockerfile", "containerize", "Dockerize this app"    | Multi-stage build from scratch   |
| **Build Optimization** | "slow build", "cache issue", "rebuilds everything"          | Layer ordering, cache mounts     |
| **Image Size**      | "image too large", "reduce size", "minimize image"             | Distroless, multi-stage, cleanup |
| **Security**        | "vulnerabilities", "security scan", "run as non-root"          | Non-root user, secrets, minimal  |
| **Compose Setup**   | "docker-compose", "orchestrate", "multi-service"               | Services, networks, health check |
| **Networking**      | "can't connect", "service not found", "DNS resolution"         | Network config, service discovery|
| **Review**          | "review my Docker", "production ready?", "best practices"      | Full checklist audit             |
| **Development**     | "hot reload", "dev container", "debug in Docker"               | Dev targets, volume mounts       |

#### Common Anti-Patterns to Spot Immediately

- **Root user without USER directive** — security risk, fix first
- **Source code copied before dependencies** — kills layer caching, slow builds
- **`npm install` instead of `npm ci`** — non-deterministic builds
- **Secrets in ENV or COPY** — exposed in image layers
- **No .dockerignore** — bloated build context, slow transfers
- **`latest` tag in base image** — non-reproducible builds
- **Multiple services in one container** — violates single-responsibility principle

### Phase 3: Solution Design — Core Patterns

**Trigger:** After problem categorization. Apply the appropriate pattern based on the user's stack.

#### Pattern 1: Multi-Stage Build (Universal Starting Point)

This is the foundation for almost every production Dockerfile. Separate build dependencies from runtime artifacts:

```dockerfile
# Stage 1: Install dependencies (cache-friendly)
FROM node:18-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production && npm cache clean --force

# Stage 2: Build the application
FROM node:18-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build && npm prune --production

# Stage 3: Production runtime (minimal)
FROM node:18-alpine AS runtime
RUN addgroup -g 1001 -S nodejs && adduser -S appuser -u 1001 -G nodejs
WORKDIR /app
COPY --from=deps --chown=appuser:nodejs /app/node_modules ./node_modules
COPY --from=build --chown=appuser:nodejs /app/dist ./dist
COPY --from=build --chown=appuser:nodejs /app/package*.json ./
USER appuser
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:3000/health || exit 1
CMD ["node", "dist/index.js"]
```

**Key principles in this pattern:**
- `deps` stage isolates dependency installation → only rebuilds when package.json changes
- `build` stage has full toolchain → separated from runtime
- `runtime` stage is minimal → no build tools, non-root user, health check included
- `--chown` on COPY ensures correct file ownership with non-root user

#### Pattern 2: Language-Specific Optimizations

Adapt the multi-stage pattern for each language ecosystem:

**Go — use scratch or distroless (no runtime needed):**
```dockerfile
FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/server .

FROM gcr.io/distroless/static-debian12
COPY --from=build /app/server /server
USER nonroot:nonroot
EXPOSE 8080
CMD ["/server"]
```

**Python — use virtualenv and slim base:**
```dockerfile
FROM python:3.12-slim AS build
WORKDIR /app
COPY requirements.txt .
RUN pip install --user --no-cache-dir -r requirements.txt

FROM python:3.12-slim AS runtime
RUN useradd -m -u 1001 appuser
COPY --from=build --chown=appuser:appuser /root/.local /home/appuser/.local
COPY --chown=appuser:appuser . .
USER appuser
ENV PATH=/home/appuser/.local/bin:$PATH
EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
```

**Java/Maven — build in one stage, run JRE in another:**
```dockerfile
FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline -B
COPY src ./src
RUN mvn package -DskipTests

FROM eclipse-temurin:21-jre-alpine AS runtime
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=build --chown=appuser:appgroup /app/target/*.jar /app/app.jar
USER appuser
EXPOSE 8080
CMD ["java", "-jar", "/app/app.jar"]
```

#### Pattern 3: Docker Compose Production Setup

```yaml
version: '3.8'
services:
  app:
    build:
      context: .
      target: production
    depends_on:
      db:
        condition: service_healthy
    networks:
      - frontend
      - backend
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M
    restart: unless-stopped

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB_FILE: /run/secrets/db_name
      POSTGRES_USER_FILE: /run/secrets/db_user
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    secrets:
      - db_name
      - db_user
      - db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    networks:
      - backend
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$(cat /run/secrets/db_user)"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge
    internal: true

volumes:
  postgres_data:

secrets:
  db_name:
    external: true
  db_user:
    external: true
  db_password:
    external: true
```

**Key decisions in this compose file:**
- `condition: service_healthy` ensures database is ready before app starts
- `backend` network is `internal: true` — database not exposed to external traffic
- Secrets use `_FILE` variants — never in environment variables directly
- Resource limits prevent any one service from starving others
- `restart: unless-stopped` for production resilience

#### Pattern 4: Development Environment Override

Use a separate compose override file for development:

```yaml
# docker-compose.override.yml (development)
services:
  app:
    build:
      target: development
    volumes:
      - .:/app
      - /app/node_modules
    environment:
      - NODE_ENV=development
      - DEBUG=app:*
    ports:
      - "9229:9229"
    command: npm run dev
```

Or use a profile-based approach in a single compose file:

```yaml
services:
  app:
    build:
      context: .
      target: ${BUILD_TARGET:-production}
    volumes:
      - ${DEV_VOLUME:-}  # Only set in .env.dev
    profiles:
      - ${PROFILE:-production}
```

#### Pattern 5: Security Hardening Checklist

Every production Dockerfile must include these security measures. Apply them in priority order:

| Priority | Measure                         | How to Implement                                      |
| -------- | ------------------------------- | ----------------------------------------------------- |
| 🔴 P0    | Non-root user                   | `USER 1001` with explicit UID/GID creation            |
| 🔴 P0    | No secrets in image layers      | BuildKit secrets mount or runtime secrets only         |
| 🟠 P1    | Minimal base image              | Alpine or distroless, not full OS images               |
| 🟠 P1    | Pinned base image digests       | `FROM node:18-alpine@sha256:...` instead of tags      |
| 🟡 P2    | HEALTHCHECK defined             | HTTP endpoint or process check                         |
| 🟡 P2    | Drop Linux capabilities         | `--cap-drop=ALL --cap-add=NET_BIND_SERVICE` at minimum |
| 🟢 P3    | Read-only root filesystem       | `--read-only` with tmpfs for writable paths            |
| 🟢 P3    | No package manager in runtime   | Only copy built artifacts to production stage          |

**BuildKit secrets example (never leaves a layer):**
```dockerfile
# syntax=docker/dockerfile:1
FROM node:18-alpine
RUN --mount=type=secret,id=npm_token \
    NPM_TOKEN=$(cat /run/secrets/npm_token) \
    npm ci --only=production
```

#### Pattern 6: Build Cache Optimization with BuildKit

```dockerfile
# syntax=docker/dockerfile:1
FROM node:18-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --only=production
```

For package managers with cache directories:

| Package Manager | Cache Mount Target    |
| --------------- | --------------------- |
| npm             | `/root/.npm`          |
| yarn            | `/usr/local/share/.cache/yarn` |
| pip             | `/root/.cache/pip`    |
| go modules      | `/go/pkg/mod`         |
| maven           | `/root/.m2`           |
| apt             | `/var/cache/apt`      |

### Phase 4: Implementation

**Trigger:** After solution design is confirmed. Write or modify files.

#### Step 4.1 - Determine File Creation Strategy

| Scenario                                 | Action                                              |
| ---------------------------------------- | --------------------------------------------------- |
| No Dockerfile exists                     | Create `Dockerfile` from scratch                    |
| Existing Dockerfile needs optimization   | Use `SearchReplace` to modify specific sections     |
| Multiple services need orchestration     | Create or modify `docker-compose.yml`               |
| Build context is slow                    | Create or update `.dockerignore`                    |

#### Step 4.2 - .dockerignore Template

Always ensure a comprehensive `.dockerignore` exists. This dramatically speeds up builds by reducing context size:

```
node_modules
.git
.gitignore
*.md
.git
.env
.env.*
dist
build
coverage
.nyc_output
*.log
.DS_Store
.vscode
.idea
docker-compose*.yml
Dockerfile*
```

Adapt to the project's stack — exclude test files, documentation, and any directory that gets regenerated in the build.

#### Step 4.3 - Layer Ordering Principles

When writing or modifying a Dockerfile, always respect this ordering:

```
1. Base image (FROM)              — changes rarely
2. System dependencies (apt/apk)  — changes rarely
3. Package manager files (COPY)   — changes occasionally
4. Dependency installation (RUN)  — changes when deps change
5. Application source (COPY)      — changes frequently
6. Build steps (RUN)              — changes frequently
7. Runtime configuration          — changes occasionally
```

**The golden rule:** Everything that changes rarely goes FIRST. Everything that changes frequently goes LAST.

#### Common Layer Ordering Mistakes

**Wrong (source before dependencies):**
```dockerfile
COPY . .
RUN npm ci
```
Every code change invalidates the npm cache → re-downloads everything.

**Correct (dependencies before source):**
```dockerfile
COPY package*.json ./
RUN npm ci
COPY . .
```
Only re-runs `npm ci` when `package.json` changes.

### Phase 5: Validation

**Trigger:** After every Dockerfile or compose file change. Always validate before considering the task complete.

#### Step 5.1 - Build Validation

```bash
docker build --no-cache -t test-build .
```

If the build fails:
- Check base image availability (`docker pull <image>`)
- Verify COPY paths exist in the build context
- Check for syntax errors in RUN commands
- Ensure multi-stage COPY --from targets exist

#### Step 5.2 - Image Size Inspection

```bash
docker images test-build --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
docker history test-build --no-trunc --format "table {{.Size}}\t{{.CreatedBy}}" | head -10
```

| Size Concern      | Threshold | Action                                    |
| ----------------- | --------- | ----------------------------------------- |
| Acceptable        | < 500MB   | Good for most applications                |
| Needs Attention   | 500MB-1GB | Check for build tools or caches in image  |
| Too Large         | > 1GB     | Apply multi-stage build, switch to distroless |

#### Step 5.3 - Security Scan (if available)

```bash
docker scout quickview test-build 2>/dev/null || echo "Docker Scout not available"
```

If Docker Scout is not available, recommend the user runs a scan with their preferred tool.

#### Step 5.4 - Runtime Verification

```bash
docker run --rm -d --name validate-test test-build
sleep 5
docker ps --filter name=validate-test --format "{{.Status}}"
docker logs validate-test --tail 20
docker stop validate-test
```

Check for:
- Container starts without immediate crash
- Application binds to the expected port
- Health check passes (if defined)
- No permission errors (correct USER setup)
- Logs show expected startup messages

#### Step 5.5 - Compose Validation (if applicable)

```bash
docker-compose config 2>/dev/null && echo "Compose config valid"
docker-compose up -d --build
docker-compose ps
docker-compose down
```

## Reference: Code Review Checklist

When reviewing existing Docker configurations, check each item below. Report findings organized by severity:

### Dockerfile Quality
- [ ] Dependencies copied before source code for optimal layer caching
- [ ] Multi-stage builds separate build and runtime environments
- [ ] Production stage only includes necessary artifacts
- [ ] Build context optimized with comprehensive .dockerignore
- [ ] Base image selection appropriate for the stack and constraints
- [ ] RUN commands consolidated to minimize layers where beneficial

### Container Security
- [ ] Non-root user created with specific UID/GID (not default)
- [ ] Container runs as non-root user (USER directive present)
- [ ] Secrets managed properly (not in ENV vars or image layers)
- [ ] Base images kept up-to-date and scanned for vulnerabilities
- [ ] Minimal attack surface (only necessary packages installed)
- [ ] Health checks implemented for container monitoring

### Docker Compose & Orchestration
- [ ] Service dependencies properly defined with health checks
- [ ] Custom networks configured for service isolation
- [ ] Environment-specific configurations separated (dev/prod)
- [ ] Volume strategies appropriate for data persistence needs
- [ ] Resource limits defined to prevent resource exhaustion
- [ ] Restart policies configured for production resilience

### Performance & Size
- [ ] Final image size under 500MB (unless justified)
- [ ] Build cache optimization implemented (mounts or layer ordering)
- [ ] Multi-architecture builds considered if needed
- [ ] Artifact copying selective (only required files)
- [ ] Package manager cache cleaned in same RUN layer

### Development Workflow
- [ ] Development targets separate from production
- [ ] Hot reloading configured properly with volume mounts
- [ ] Debug ports exposed when needed
- [ ] Environment variables properly configured for different stages
- [ ] Testing containers isolated from production builds

## Reference: Troubleshooting Common Issues

### Build Performance Issues
**Symptoms**: Slow builds (10+ minutes), frequent cache invalidation
**Root causes**: Poor layer ordering, large build context, no caching strategy
**Fix**: Reorder layers (deps before source), add .dockerignore, use BuildKit cache mounts

### Security Vulnerabilities
**Symptoms**: Security scan failures, exposed secrets, root execution
**Root causes**: Outdated base images, hardcoded secrets, default user
**Fix**: Pin base image digests, use BuildKit secrets, add non-root USER directive

### Image Size Problems
**Symptoms**: Images over 1GB, deployment slowness
**Root causes**: Unnecessary files, build tools in production, poor base selection
**Fix**: Switch to distroless, multi-stage optimization, selective artifact copying

### Networking Issues
**Symptoms**: Service communication failures, DNS resolution errors
**Root causes**: Missing networks, port conflicts, service naming
**Fix**: Define custom networks, add health checks with `depends_on: condition: service_healthy`

### Development Workflow Problems
**Symptoms**: Hot reload failures, debugging difficulties, slow iteration
**Root causes**: Volume mounting issues, port configuration, environment mismatch
**Fix**: Create development-specific targets, proper volume strategy, debug configuration

## Reference: Integration Boundaries

**When to recommend other skills/experts:**
- **Kubernetes pods, services, ingress** → Not in scope. Docker runs single containers; K8s orchestrates clusters.
- **CI/CD pipeline with containers** → Combine this skill with GitHub Actions or CI platform expertise.
- **Cloud-specific container services (ECS, Fargate, Cloud Run)** → Docker patterns apply, but deployment specifics need cloud expertise.
- **Database containerization with complex persistence** → Basic patterns here; complex backup/HA strategies need database expertise.
