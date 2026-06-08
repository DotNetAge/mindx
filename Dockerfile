# =============================================================================
# MindX Docker Image — Alpine (musl) base, matches musl-cross compiled binary
# =============================================================================
# Build:
#   Local:  make docker  →  pre-compile + docker compose build
#   CI:     build job   →  cross-compile artifacts → docker build-push
#
# Binary source: runtime/bin/mindx (pre-compiled, NOT built inside Docker)
# =============================================================================

# ---- Runtime image ----
FROM alpine:3.19

LABEL maintainer="DotNetAge <ray@raya.cn>"
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

# Deploy runtime environment + pre-built binary
COPY --chown=mindx:mindx runtime/ /home/mindx/.mindx/

# Ensure binary is executable
RUN [ -f /home/mindx/.mindx/bin/mindx ] && chmod +x /home/mindx/.mindx/bin/mindx || true

# Runtime directories
RUN mkdir -p /home/mindx/.mindx/logs \
    /home/mindx/.mindx/sessions

# Python venv
RUN python3 -m venv /home/mindx/.mindx/.venv
RUN /home/mindx/.mindx/.venv/bin/pip freeze > /home/mindx/.mindx/requirements.txt
RUN /home/mindx/.mindx/.venv/bin/pip install -r /home/mindx/.mindx/requirements.txt

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
