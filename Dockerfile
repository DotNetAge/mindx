# =============================================================================
# Single-stage: 打包预编译的二进制 + runtime/ 配置环境
# 先运行 make build-linux-amd64 再将二进制复制到 runtime/bin/mindx
# =============================================================================
FROM ubuntu:22.04

# 安装系统工具 + Python + Node.js
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        python3 \
        python3-venv \
        python3-pip \
        tini \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# 创建非 root 用户
RUN useradd -m -s /bin/bash mindx
USER mindx
WORKDIR /home/mindx

# 部署预配置的运行环境（含预编译的 binary）runtime/ → ~/.mindx/
COPY --chown=mindx:mindx runtime/ /home/mindx/.mindx/

# 创建运行时所需目录
RUN mkdir -p /home/mindx/.mindx/logs \
             /home/mindx/.mindx/sessions

# Python 虚拟环境
RUN python3 -m venv /home/mindx/.mindx/.venv

# 修正 mindx.json 中的 venv_path
RUN sed -i 's|/Users/ray/.mindx/.venv|/home/mindx/.mindx/.venv|g' \
       /home/mindx/.mindx/mindx.json

# 工作区目录（与宿主机共享）
RUN mkdir -p /home/mindx/workspaces

# 端口
# 1314: WebSocket 网关
# 1313: Web UI
EXPOSE 1314 1313

# 入口
ENTRYPOINT ["/usr/bin/tini", "--", "/home/mindx/.mindx/bin/mindx"]
CMD ["start"]
