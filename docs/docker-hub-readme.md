# MindX² — AI-Native Multi-Agent Operating System

MindX² is an AI-native multi-agent conversation platform that lets one person run an entire company. It plans, executes, reviews, and improves — like an experienced human, not a chatbot.

---

## Prerequisites: Install Docker

You need Docker to run MindX². If you don't have it yet:

**macOS**
```bash
brew install --cask docker
# Or download from https://docs.docker.com/desktop/setup/install/mac-install/
```

**Linux (Ubuntu/Debian)**
```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update && sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

**Windows** — Download Docker Desktop from https://docs.docker.com/desktop/setup/install/windows-install/

Verify installation:
```bash
docker --version
docker compose version
```

---

## Quick Start

### 1. Pull the Image

```bash
docker pull dotnetage/mindx:latest
```

### 2. Run with API Keys

MindX² connects to multiple LLM providers. Set your API keys as environment variables — they will be passed into the container automatically:

```bash
docker run -d \
  --name mindx \
  -p 1313:1313 \
  -p 1314:1314 \
  -e DEEPSEEK_API_KEY="your-deepseek-key" \
  -e DASHSCOPE_API_KEY="your-dashscope-key" \
  -e OPENAI_API_KEY="your-openai-key" \
  -e ANTHROPIC_API_KEY="your-anthropic-key" \
  -e ZHIPU_API_KEY="your-zhipu-key" \
  -e MOONSHOT_API_KEY="your-moonshot-key" \
  -e MINIMAX_API_KEY="your-minimax-key" \
  -e ARK_API_KEY="your-ark-key" \
  -e GOOGLE_API_KEY="your-google-key" \
  -v mindx-data:/home/mindx/.mindx/sessions \
  -v mindx-logs:/home/mindx/.mindx/logs \
  dotnetage/mindx:latest
```

Only set the keys for the providers you plan to use — the rest can be omitted.

### 3. Open the Web UI

Visit [http://localhost:1313](http://localhost:1313) in your browser.

---

## Using Docker Compose (Recommended)

### How API Keys Work

If you already have API keys set on your host machine (e.g., in `~/.zshrc`, `~/.bashrc`, or a `.env` file), **Docker Compose will pick them up automatically**. No need to hardcode them in any file.

For example, if your host has:
```bash
export DEEPSEEK_API_KEY=sk-xxx
export OPENAI_API_KEY=sk-xxx
```

Compose injects these into the container automatically via the `environment` section.

### docker-compose.yml

Below is the full configuration. Save it as `docker-compose.yml` in any directory:

```yaml
services:
  mindx:
    build:
      context: .
      dockerfile: Dockerfile
      args:
        VERSION: ${MINDX_VERSION:-dev}
        COMMIT: ${MINDX_COMMIT:-unknown}
        BUILD_TIME: ${MINDX_BUILD_TIME:-unknown}
    image: mindx:${MINDX_VERSION:-dev}
    container_name: ${COMPOSE_PROJECT_NAME:-mindx}-daemon
    ports:
      - "${MINDX_WEB_PORT:-1313}:1313"
      - "${MINDX_WS_PORT:-1314}:1314"
    environment:
      - DEEPSEEK_API_KEY=${DEEPSEEK_API_KEY:-}
      - DASHSCOPE_API_KEY=${DASHSCOPE_API_KEY:-}
      - OPENAI_API_KEY=${OPENAI_API_KEY:-}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}
      - ZHIPU_API_KEY=${ZHIPU_API_KEY:-}
      - MOONSHOT_API_KEY=${MOONSHOT_API_KEY:-}
      - MINIMAX_API_KEY=${MINIMAX_API_KEY:-}
      - ARK_API_KEY=${ARK_API_KEY:-}
      - GOOGLE_API_KEY=${GOOGLE_API_KEY:-}
    volumes:
      - mindx-data:/home/mindx/.mindx/sessions
      - mindx-logs:/home/mindx/.mindx/logs
      - ./workspaces:/home/mindx/workspaces
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:1313/"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3

volumes:
  mindx-data:
    name: ${COMPOSE_PROJECT_NAME:-mindx}-data
  mindx-logs:
    name: ${COMPOSE_PROJECT_NAME:-mindx}-logs
```

### Basic Compose Commands

```bash
# Start MindX² in the background
docker compose up -d

# View logs
docker compose logs -f

# Stop and remove
docker compose down

# Restart
docker compose restart
```

---

## Supported LLM Providers

| Provider     | Env Variable        | Get Your Key                     |
|-------------|---------------------|----------------------------------|
| DeepSeek    | `DEEPSEEK_API_KEY`  | https://platform.deepseek.com    |
| DashScope   | `DASHSCOPE_API_KEY` | https://dashscope.aliyun.com     |
| OpenAI      | `OPENAI_API_KEY`    | https://platform.openai.com      |
| Anthropic   | `ANTHROPIC_API_KEY` | https://console.anthropic.com    |
| Zhipu       | `ZHIPU_API_KEY`     | https://open.bigmodel.cn         |
| Moonshot    | `MOONSHOT_API_KEY`  | https://platform.moonshot.cn     |
| MiniMax     | `MINIMAX_API_KEY`   | https://platform.minimaxi.com    |
| Ark         | `ARK_API_KEY`       | https://console.volcengine.com   |
| Google      | `GOOGLE_API_KEY`    | https://aistudio.google.com      |

---

## Volumes

| Volume          | Mount Point                        | Purpose              |
|-----------------|-----------------------------------|----------------------|
| `mindx-data`    | `/home/mindx/.mindx/sessions`     | Persistent sessions  |
| `mindx-logs`    | `/home/mindx/.mindx/logs`         | Application logs     |
| Host `./workspaces` | `/home/mindx/workspaces`      | Shared workspace     |

---

## Ports

| Port | Protocol | Service   |
|------|----------|-----------|
| 1313 | HTTP     | Web UI    |
| 1314 | WebSocket| Gateway   |
