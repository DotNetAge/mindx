---
source: https://github.com/anthropics/skills/tree/main/skills/skill-creator
updated: 2026-10-11
---
## 目录

- 一次性命令 [L22-L106]
- 从 `SKILL.md` 引用脚本 [L107-L139]
- 自包含脚本 [L140-L245]
- 为智能体使用设计脚本 [L246-L320]
  - 避免交互式提示 [L250-L266]
  - 用 `--help` 说明用法 [L267-L287]
  - 编写有用的错误信息 [L288-L296]
  - 使用结构化输出 [L297-L311]
  - 其他考虑 [L312-L320]
# 在 skill 中使用脚本

> 如何在你的 skill 中运行命令并打包可执行脚本。

skill 可以指示智能体运行 shell 命令，并在 `scripts/` 目录中打包可复用脚本。本指南涵盖一次性命令、自带依赖的自包含脚本，以及如何为智能体使用设计脚本接口。

## 一次性命令

当现有包已经能完成所需工作时，你可以在 `SKILL.md` 指令中直接引用它，无需 `scripts/` 目录。许多生态提供了运行时自动解析依赖的工具。

<Tabs sync={false}>
  <Tab title="uvx">
    [uvx](https://docs.astral.sh/uv/guides/tools/) 在隔离环境中运行 Python 包，带激进缓存。它随 [uv](https://docs.astral.sh/uv/) 一起发布。

    ```bash theme={null}
    uvx ruff@0.8.0 check .
    uvx black@24.10.0 .
    ```

    * 不随 Python 附带——需要单独安装。
    * 快。激进缓存让重复运行近乎即时。
  </Tab>

  <Tab title="pipx">
    [pipx](https://pipx.pypa.io/) 在隔离环境中运行 Python 包。可通过操作系统包管理器获得（`apt install pipx`、`brew install pipx`）。

    ```bash theme={null}
    pipx run 'black==24.10.0' .
    pipx run 'ruff==0.8.0' check .
    ```

    * 不随 Python 附带——需要单独安装。
    * `uvx` 的成熟替代品。虽然 `uvx` 已成为标准推荐，`pipx` 仍是可靠选择，且操作系统包管理器支持更广。
  </Tab>

  <Tab title="npx">
    [npx](https://docs.npmjs.com/cli/commands/npx) 运行 npm 包，按需下载。它随 npm 一起发布（npm 随 Node.js 一起发布）。

    ```bash theme={null}
    npx eslint@9 --fix .
    npx create-vite@6 my-app
    ```

    * 随 Node.js 附带——无需额外安装。
    * 下载包、运行并缓存以备后用。
    * 用 `npx package@version` 固定版本以保证可复现。
  </Tab>

  <Tab title="bunx">
    [bunx](https://bun.sh/docs/cli/bunx) 是 Bun 版的 `npx`。它随 [Bun](https://bun.sh/) 一起发布。

    ```bash theme={null}
    bunx eslint@9 --fix .
    bunx create-vite@6 my-app
    ```

    * 在基于 Bun 的环境中可作为 `npx` 的即插即用替代。
    * 仅当用户环境使用 Bun 而非 Node.js 时才适用。
  </Tab>

  <Tab title="deno run">
    [deno run](https://docs.deno.com/runtime/reference/cli/run/) 直接从 URL 或说明符运行脚本。它随 [Deno](https://deno.com/) 一起发布。

    ```bash theme={null}
    deno run npm:create-vite@6 my-app
    deno run --allow-read npm:eslint@9 -- --fix .
    ```

    * 文件系统/网络访问需要权限标志（`--allow-read` 等）。
    * 用 `--` 分隔 Deno 标志与工具自身的标志。
  </Tab>

  <Tab title="go run">
    [go run](https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program) 直接编译并运行 Go 包。它内置于 `go` 命令。

    ```bash theme={null}
    go run golang.org/x/tools/cmd/goimports@v0.28.0 .
    go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.0 run
    ```

    * 内置于 Go——无需额外工具。
    * 固定版本或使用 `@latest` 让命令更明确。
  </Tab>
</Tabs>

**skill 中一次性命令的技巧：**

* **固定版本**（如 `npx eslint@9.0.0`），让命令随时间表现一致。
* **在 `SKILL.md` 中说明前提**（如"需要 Node.js 18+"），不要假设智能体环境里有它们。运行时级的要求，使用[`compatibility` 前置元数据字段](spec.md#兼容性字段)。
* **把复杂命令移入脚本。** 当你用几个标志调用一个工具时，一次性命令很合适。当命令复杂到第一次就写对的难度很大时，`scripts/` 里经过测试的脚本更可靠。

## 从 `SKILL.md` 引用脚本

使用**相对于 skill 目录根目录的路径**引用打包文件。智能体会自动解析这些路径——无需绝对路径。

在 `SKILL.md` 中列出可用脚本，让智能体知道它们存在：

```markdown SKILL.md
## 可用脚本

- **`scripts/validate.sh`** — 验证配置文件
- **`scripts/process.py`** — 处理输入数据
```

然后指示智能体运行它们：

````markdown SKILL.md
## 工作流

1. 运行验证脚本：
   ```bash
   bash scripts/validate.sh "$INPUT_FILE"
   ```

2. 处理结果：
   ```bash
   python3 scripts/process.py --input results.json
   ```
````

<Note>
  相同的相对路径约定适用于 `references/*.md` 等支持文件——脚本执行路径（代码块中）相对于**skill 目录根目录**，因为智能体从那里运行命令。
</Note>

## 自包含脚本

当需要可复用逻辑时，在 `scripts/` 中打包一个在文件内声明自身依赖的脚本。智能体可以用单条命令运行该脚本——无需单独的清单文件或安装步骤。

多种语言支持内联依赖声明：

<Tabs sync={false}>
  <Tab title="Python">
    [PEP 723](https://peps.python.org/pep-0723/) 定义了内联脚本元数据的标准格式。在 `# ///` 标记内的 TOML 块中声明依赖：

    ```python scripts/extract.py theme={null}
    # /// script
    # dependencies = [
    #   "beautifulsoup4",
    # ]
    # ///

    from bs4 import BeautifulSoup

    html = '<html><body><h1>Welcome</h1><p class="info">This is a test.</p></body></html>'
    print(BeautifulSoup(html, "html.parser").select_one("p.info").get_text())
    ```

    用 [uv](https://docs.astral.sh/uv/) 运行（推荐）：

    ```bash theme={null}
    uv run scripts/extract.py
    ```

    `uv run` 会创建隔离环境、安装声明的依赖并运行脚本。[pipx](https://pipx.pypa.io/)（`pipx run scripts/extract.py`）也支持 PEP 723。

    * 用 [PEP 508](https://peps.python.org/pep-0508/) 说明符固定版本：`"beautifulsoup4>=4.12,<5"`。
    * 用 `requires-python` 约束 Python 版本。
    * 用 `uv lock --script` 创建锁文件以获得完全可复现性。
  </Tab>

  <Tab title="Deno">
    Deno 的 `npm:` 和 `jsr:` 导入说明符让每个脚本默认自包含：

    ```typescript scripts/extract.ts theme={null}
    #!/usr/bin/env -S deno run

    import * as cheerio from "npm:cheerio@1.0.0";

    const html = `<html><body><h1>Welcome</h1><p class="info">This is a test.</p></body></html>`;
    const $ = cheerio.load(html);
    console.log($("p.info").text());
    ```

    ```bash theme={null}
    deno run scripts/extract.ts
    ```

    * npm 包用 `npm:`，Deno 原生包用 `jsr:`。
    * 版本说明符遵循 semver：`@1.0.0`（精确）、`@^1.0.0`（兼容）。
    * 依赖全局缓存。用 `--reload` 强制重新获取。
    * 带原生插件（node-gyp）的包可能无法工作——发布预编译二进制的包效果最好。
  </Tab>

  <Tab title="Bun">
    当找不到 `node_modules` 目录时，Bun 会在运行时自动安装缺失的包。直接在导入路径中固定版本：

    ```typescript scripts/extract.ts theme={null}
    #!/usr/bin/env bun

    import * as cheerio from "cheerio@1.0.0";

    const html = `<html><body><h1>Welcome</h1><p class="info">This is a test.</p></body></html>`;
    const $ = cheerio.load(html);
    console.log($("p.info").text());
    ```

    ```bash theme={null}
    bun run scripts/extract.ts
    ```

    * 无需 `package.json` 或 `node_modules`。TypeScript 原生可用。
    * 包全局缓存。首次运行下载；后续运行近乎即时。
    * 如果目录树任何位置存在 `node_modules` 目录，自动安装会被禁用，Bun 回退到标准 Node.js 解析。
  </Tab>

  <Tab title="Ruby">
    Bundler 自 Ruby 2.6 起随 Ruby 发布。用 `bundler/inline` 直接在脚本中声明 gems：

    ```ruby scripts/extract.rb theme={null}
    require 'bundler/inline'

    gemfile do
      source 'https://rubygems.org'
      gem 'nokogiri'
    end

    html = '<html><body><h1>Welcome</h1><p class="info">This is a test.</p></body></html>'
    doc = Nokogiri::HTML(html)
    puts doc.at_css('p.info').text
    ```

    ```bash theme={null}
    ruby scripts/extract.rb
    ```

    * 明确固定版本（`gem 'nokogiri', '~> 1.16'`）——没有锁文件。
    * 工作目录中已有的 `Gemfile` 或 `BUNDLE_GEMFILE` 环境变量可能造成干扰。
  </Tab>
</Tabs>

## 为智能体使用设计脚本

当智能体运行你的脚本时，它读取 stdout 和 stderr 来决定下一步做什么。几个设计选择能大幅降低智能体使用脚本的难度。

### 避免交互式提示

这是智能体执行环境的硬性要求。智能体在非交互 shell 中运行——它们无法响应 TTY 提示、密码对话框或确认菜单。阻塞在交互输入上的脚本会无限挂起。

所有输入通过命令行标志、环境变量或标准输入接收：

```
# 差：挂起等待输入
$ python scripts/deploy.py
目标环境：_

# 好：带指引的清晰报错
$ python scripts/deploy.py
错误：缺少 --env。可选值：development、staging、production。
用法：python scripts/deploy.py --env staging --tag v1.2.3
```

### 用 `--help` 说明用法

`--help` 输出是智能体了解脚本接口的主要途径。包含简短描述、可用标志和用法示例：

```
用法：scripts/process.py [选项] 输入文件

处理输入数据并生成汇总报告。

选项：
  --format 格式      输出格式：json、csv、table（默认：json）
  --output 文件      将输出写入文件而不是 stdout
  --verbose          将进度打印到 stderr

示例：
  scripts/process.py data.csv
  scripts/process.py --format csv --output report.csv data.csv
```

保持简洁——输出会同智能体正在处理的其他一切一起进入它的上下文窗口。

### 编写有用的错误信息

当智能体遇到错误时，错误信息直接塑造它的下一次尝试。一句晦涩的"错误：输入无效"浪费一整轮。相反，说明哪里出了问题、期望什么、可以试什么：

```
错误：--format 必须是 json、csv、table 之一。
      收到："xml"
```

### 使用结构化输出

优先使用结构化格式——JSON、CSV、TSV——而不是自由文本。结构化格式可以被智能体和标准工具（`jq`、`cut`、`awk`）共同消费，让你的脚本可以在管道中组合。

```
# 空白对齐——难以编程解析
NAME          STATUS    CREATED
my-service    running   2025-01-15

# 定界——字段边界无歧义
{"name": "my-service", "status": "running", "created": "2025-01-15"}
```

**数据与诊断分离：** 结构化数据走 stdout，进度信息、警告和其他诊断信息走 stderr。这让智能体既能捕获干净、可解析的输出，又能在需要时获得诊断信息。

### 其他考虑

* **幂等性。** 智能体可能重试命令。"不存在则创建"比"重复执行就报错"更安全。
* **输入约束。** 对模糊输入给出清晰报错而非猜测。尽可能使用枚举和封闭集合。
* **试运行支持。** 对于破坏性或状态性操作，`--dry-run` 标志让智能体预览将要发生什么。
* **有意义的退出码。** 为不同失败类型（未找到、参数无效、认证失败）使用不同的退出码，并在 `--help` 输出中说明它们，让智能体知道每个码的含义。
* **安全的默认值。** 考虑破坏性操作是否需要显式确认标志（`--confirm`、`--force`）或其他与风险级别相称的保护措施。
* **可预期的输出大小。** 许多智能体 harness 会自动截断超过阈值（如 10-30K 字符）的工具输出，可能丢失关键信息。如果你的脚本可能产生大量输出，默认输出摘要或合理限制，并支持 `--offset` 等标志，让智能体在需要时请求更多信息。或者，如果输出很大且不适合分页，要求智能体传入 `--output` 标志，指定输出文件或 `-` 以显式选择 stdout。
