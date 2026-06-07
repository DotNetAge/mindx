# =============================================================================
# MindX Docker Image — Alpine (musl) base, matches musl-cross compiled binary
# =============================================================================
# Build:
#   1. Pre-compile: CGO_ENABLED=1 CC=x86_64-linux-musl-gcc GOOS=linux \
#      GOARCH=amd64 go build -o runtime/bin/mindx .
#   2. docker compose build
#
# Version injection (optional):
#   docker build --build-arg VERSION=v2.2.0 --build-arg COMMIT=abc1234 .
# =============================================================================

# ---- Build stage (optional: compile from source) ----
FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w \
      -X github.com/DotNetAge/mindx/cmd.Version=${VERSION#v} \
      -X github.com/DotNetAge/mindx/cmd.Commit=${COMMIT} \
      -X github.com/DotNetAge/mindx/cmd.BuildTime=${BUILD_TIME}" \
    -o /mindx .

# ---- Runtime stage ----
FROM alpine:3.19

LABEL maintainer="DotNetAge <ray@dotnetage.com>"
LABEL org.opencontainers.image.source="https://github.com/DotNetAge/mindx"
LABEL org.opencontainers.image.description="MindX AI-native multi-agent conversation platform"

ARG VERSION=dev
ENV MINDX_VERSION=${VERSION}

# Install runtime dependencies
RUN apk add --no-cache \
        ca-certificates \
        curl \
        git \
        python3 \
        py3-pip \
        py3-virtualenv \
        nodejs \
        tini \
        bash

# Non-root user
RUN adduser -D -s /bin/bash mindx
USER mindx
WORKDIR /home/mindx

# Deploy runtime environment + binary (prefer pre-built, fallback to builder)
COPY --chown=mindx:mindx runtime/ /home/mindx/.mindx/
COPY --from=builder --chown=mindx:mindx /mindx /home/mindx/.mindx/bin/mindx

# Ensure binary is executable
RUN chmod +x /home/mindx/.mindx/bin/mindx 2>/dev/null || true

# Runtime directories
RUN mkdir -p /home/mindx/.mindx/logs \
             /home/mindx/.mindx/sessions

# Python venv
RUN python3 -m venv /home/mindx/.mindx/.venv

# Fix venv path in mindx.json for container
RUN sed -i 's|/Users/ray/.mindx/.venv|/home/mindx/.mindx/.venv|g' \
       /home/mindx/.mindx/mindx.json 2>/dev/null || true

# Workspace directory (shared with host)
RUN mkdir -p /home/mindx/workspaces

# Ports
# 1313: Web UI (HTTP)
# 1314: WebSocket Gateway
EXPOSE 1313 1314

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:1313/ || exit 1

ENTRYPOINT ["/sbin/tini", "--", "/home/mindx/.mindx/bin/mindx"]
CMD ["start"]
