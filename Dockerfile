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

# Install runtime dependencies (minimal set)
RUN apk add --no-cache \
    ca-certificates \
    python3 \
    py3-pip \
    tini \
    bash \
    wget

# Non-root user + runtime directories (single layer)
RUN adduser -D -s /bin/bash mindx && \
    mkdir -p /home/mindx/.mindx/logs \
           /home/mindx/.mindx/sessions \
           /home/mindx/workspaces

USER mindx
WORKDIR /home/mindx

# Deploy runtime environment + pre-built binary
COPY --chown=mindx:mindx runtime/ /home/mindx/.mindx/

# Ensure binary is executable
RUN [ -f /home/mindx/.mindx/bin/mindx ] && chmod +x /home/mindx/.mindx/bin/mindx || true

# Python venv (only install if requirements.txt exists with content)
RUN python3 -m venv /home/mindx/.mindx/.venv && \
    if [ -s /home/mindx/.mindx/requirements.txt 2>/dev/null ]; then \
        /home/mindx/.mindx/.venv/bin/pip install --no-cache-dir -r /home/mindx/.mindx/requirements.txt && \
        /home/mindx/.mindx/.venv/bin/pip cache purge; \
    fi

# Fix venv path in mindx.json for container
RUN sed -i 's|/Users/ray/.mindx/.venv|/home/mindx/.mindx/.venv|g' \
    /home/mindx/.mindx/mindh.json 2>/dev/null || true

# Ports
# 1313: Web UI (HTTP)
# 1314: WebSocket Gateway
EXPOSE 1313 1314

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:1313/ || exit 1

ENTRYPOINT ["/sbin/tini", "--", "/home/mindx/.mindx/bin/mindx"]
CMD ["start"]
