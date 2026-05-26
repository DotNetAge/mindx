# Release Pipeline 发布流水线

```mermaid
flowchart TD
    subgraph Build["🛠 编译 (make cross-build)"]
        D1["darwin/amd64<br/>21MB"] --> P1
        D2["darwin/arm64<br/>20MB"] --> P2
        L1["linux/amd64<br/>21MB"] --> P3
        L2["linux/arm64<br/>20MB"] --> P4
        W1["windows/amd64<br/>(受限*)"]
    end

    subgraph Package["📦 打包 (make release)"]
        P1["mindx-{V}-darwin-amd64.tar.gz"]
        P2["mindx-{V}-darwin-arm64.tar.gz"]
        P3["mindx-{V}-linux-amd64.tar.gz"]
        P4["mindx-{V}-linux-arm64.tar.gz"]
        P5["mindx-{V}-windows-amd64.zip"] --> SKIP
        SKIP["⚠ 跳过 (hnsw/renameio 上游依赖问题)"]
    end

    subgraph Checksums["🔐 校验"]
        CS["shasum -a 256 → checksums.txt"]
    end

    subgraph Publish["🚀 发布 (make release-publish)"]
        GR["gh release create v{V}<br/>上传 artifacts<br/>github.com/DotNetAge/mindx/releases"]
    end

    subgraph Homebrew["🍺 Homebrew (make release-homebrew)"]
        RB["生成 Mindx formula<br/>sha256: darwin-amd64 / arm64"] --> TAP
        TAP["推送到 DotNetAge/homebrew-tap<br/>brew install DotNetAge/mindx"]
    end

    P1 --> CS --> GR --> TAP
    P2 --> CS
    P3 --> CS
    P4 --> CS

    style W1 fill:#ffd,stroke:#a80
    style SKIP fill:#fdd,stroke:#a00
    style GR fill:#dfd,stroke:#090
    style TAP fill:#dfd,stroke:#090
```

```mermaid
flowchart TD
    PRE{"make publish<br/>分支=main?<br/>工作区干净?<br/>gh 已登录?"} -->|❌| FAIL["中止"]
    PRE -->|✅| VER["自动 bump patch<br/>v1.0.4 → v1.0.5<br/>确认?"]
    VER -->|N| CANCEL["取消"]
    VER -->|Y| TAG["git tag v1.0.5<br/>git push origin v1.0.5"]
    TAG --> BUILD["🛠 make release<br/>编译 5 平台 + 打包 + checksums"]
    BUILD --> GH["🚀 make release-publish<br/>gh release create<br/>上传 artifacts"]
    GH --> HB["🍺 make release-homebrew<br/>生成 formula<br/>推送到 homebrew-tap"]
    HB --> WG{"提交 winget<br/>PR?"}
    WG -->|Y| WINGET["🪟 make release-winget<br/>fork microsoft/winget-pkgs<br/>创建 manifest + PR"]
    WINGET --> DK
    WG -->|N| DK{"推送 Docker<br/>镜像?"}
    DK -->|Y| DI["docker build + push"]
    DK -->|N| DONE["🎉 发布完成"]
    DI --> DONE

    style PRE fill:#ddf,stroke:#66f
    style FAIL fill:#fdd,stroke:#a00
    style DONE fill:#dfd,stroke:#090
    style TAG fill:#ffd,stroke:#a80
```

## Makefile Targets

| Target | 命令 | 说明 |
|---|---|---|
| `publish` | `make publish` | 🚀 **一键发布** — 打标签 → 编译 → GitHub Release → Homebrew → Winget |
| `release` | `make release` | 编译 + 打包 + 校验和 |
| `release-notes` | `make release-notes` | 生成 RELEASE_NOTES.md（含 SHA256） |
| `release-publish` | `make release-publish` | 上传 artifacts 到 GitHub Releases |
| `release-homebrew` | `make release-homebrew` | 生成 formula → 推送到 Homebrew tap |
| `release-winget` | `make release-winget` | fork winget-pkgs → manifest → PR |

## 产物

| 平台 | 格式 | 大小 |
|---|---|---|
| macOS Intel | .tar.gz | ~21MB |
| macOS Apple Silicon | .tar.gz | ~20MB |
| Linux x86_64 | .tar.gz | ~21MB |
| Linux ARM64 | .tar.gz | ~20MB |
| Windows x86_64 | .zip | ~18MB |

## 一键发布

```bash
make publish GITHUB_REPO=DotNetAge/mindx HOMEBREW_TAP=DotNetAge/homebrew-tap
```

流程如下：
1. 检查 `main` 分支、工作区干净、`gh` 已登录
2. 自动读取最新 tag 并 bump patch 版本号（v1.0.4 → v1.0.5）
3. 交互确认后创建并推送 git tag
4. 交叉编译 5 平台（darwin + linux + windows）
5. 打包 + checksums
6. 创建 GitHub Release 并上传 artifacts
7. 生成 Homebrew formula 并推送到 tap 仓库
8. 可选提交 winget-pkgs PR → `winget install DotNetAge.Mindx`
9. 可选推送 Docker 镜像
