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
  /home/mindx/credentials \
  /home/mindx/workspaces && \
  chown -R mindx:mindx /home/mindx/.mindx /home/mindx/credentials /home/mindx/workspaces

USER mindx
WORKDIR /home/mindx

# Deploy runtime environment (agents, schemas, settings, skills, mindx.json)
COPY --chown=mindx:mindx runtime/ /home/mindx/.mindx/

# 容器内虚拟环境路径：种子文件 venv_path 留空（它是所有平台的初装模板，
# 写死容器路径会让 macOS/WSL 用户拿到一个不存在的目录），容器路径只在这里写入。
# 旧实现是一条匹配不到任何内容的 sed 加 `|| true`——永不生效，还掩盖了
# 种子被容器化的事实。现在直接写目标值，并断言写进去了。
RUN python3 -c 'import json;p="/home/mindx/.mindx/mindx.json";c=json.load(open(p));c.setdefault("python",{})["venv_path"]="/home/mindx/.mindx/.venv";json.dump(c,open(p,"w"),ensure_ascii=False,indent=2)' \
    && grep -q '/home/mindx/.mindx/.venv' /home/mindx/.mindx/mindx.json

# Ports
# 1314: WebSocket Gateway
EXPOSE 1314

ENTRYPOINT ["/usr/bin/tini", "--", "mindx"]
CMD ["daemon"]
