# =============================================================================
# MindX Docker Image — Multi-stage build
# =============================================================================
# Build:
#   Local:  make docker                (auto-build .env + docker compose)
#   Local:  docker build -t mindx .    (standalone)
#   CI:     docker/build-push-action   (multi-platform via buildx)
# =============================================================================

# ---- Builder stage ----
FROM golang:1.26-bookworm AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev \
  && rm -rf /var/lib/apt/lists/*

RUN CGO_ENABLED=1 go build \
  -trimpath \
  -ldflags="-s -w \
  -X github.com/DotNetAge/mindx/internal/core.Version=${VERSION#v} \
  -X github.com/DotNetAge/mindx/internal/core.Commit=${COMMIT} \
  -X github.com/DotNetAge/mindx/internal/core.BuildTime=${BUILD_TIME}" \
  -o /usr/local/bin/mindx \
  .

# ---- Runtime stage ----
FROM debian:bookworm-slim

LABEL maintainer="DotNetAge <ray@rayainfo.cn>"
LABEL org.opencontainers.image.source="https://github.com/DotNetAge/mindx"
LABEL org.opencontainers.image.description="MindX AI-native multi-agent conversation platform"

ARG VERSION=dev
ENV MINDX_VERSION=${VERSION}

# Install runtime dependencies (minimal set)
RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  bash \
  wget \
  tini \
  python3 \
  python3-pip \
  nodejs \
  npm \
  && rm -rf /var/lib/apt/lists/*

# Copy binary
COPY --from=builder /usr/local/bin/mindx /usr/local/bin/mindx

# Non-root user + runtime directories
RUN adduser --disabled-password --gecos '' mindx && \
  mkdir -p /home/mindx/.mindx/logs \
  /home/mindx/.mindx/sessions \
  /home/mindx/workspaces && \
  chown -R mindx:mindx /home/mindx/.mindx /home/mindx/workspaces

USER mindx
WORKDIR /home/mindx

# Deploy runtime environment (agents, schemas, settings, skills, mindx.json)
COPY --chown=mindx:mindx runtime/ /home/mindx/.mindx/

# Fix venv path in mindx.json for container
RUN sed -i 's|/Users/ray/.mindx/.venv|/home/mindx/.mindx/.venv|g' \
  /home/mindx/.mindx/mindx.json 2>/dev/null || true

# Ports
# 1314: WebSocket Gateway
EXPOSE 1314

ENTRYPOINT ["/usr/bin/tini", "--", "mindx"]
CMD ["daemon"]
