---
name: market-publish
description: >
  把本地技能、智能体、团队、插件发布到 MindX 资源社区：发布前校验、版本冲突处置、
  衍生谱系声明、组织源发布、发布后回读核对。当用户要「发布到社区、上架、发到市场、
  共享给组织、更新已发布版本、让别人能装到我的包」时使用；
  触发词：发布包、上架、发市场、共享技能、提交版本、发出去。
metadata:
  name_zh: 发布到资源社区
  version: 1.1.1
  author: Ray
  requires:
    bins:
      - mindx
      - python3
---

# 发布到资源社区

发布链路只有一条：**本机实体 → 打包 → 上传 → 服务端落库**。校验与版本纪律全部由
`scripts/release_check.py` 判定，本技能负责按判定结果推进，不凭推理放行。

## 何时使用

- 用户要把自己做的技能 / 智能体 / 团队 / 插件放到资源社区让别人安装。
- 已发布过，要更新版本、改发布名、声明衍生谱系、发布到组织专用源。
- 用户说「发布失败了」「403 包名冲突」「409 版本已存在」。

不适用：发布文章（走 `article-publish`）；官网部署（走 market-release）。

## 决策路由

按实体类型分流，四类通道不同：

| 实体 | 通道 | 命令 |
| --- | --- | --- |
| 技能 skill | daemon 打包上传 | `mindx market publish skill <name> --version <v>` |
| 智能体 agent | daemon 打包上传 | `mindx market publish agent <name> --version <v>` |
| 团队 team | daemon 打包上传 | `mindx market publish team <name> --version <v>` |
| 插件 plugin | daemon 打包上传 | `mindx market publish plugin <id> --version <v>` |

插件包的版本权威是**本机已装产物** `~/.mindx/plugins/<id>/<激活版本>/manifest.json`：发布时整包重打，只改 version 与 fingerprint（指纹 = author/id/version 三键，与服务端同算）。源码改动要先 `mw plugin pack <目录>` + `mw plugin install` 落成本机产物，再发布——直接从源码目录发会把未安装的草稿推上去。

**批量上架（仓库链路）仍然可用**：`mw plugin pack` 产 `.mindpkg` → 落 `mindx-market/contents/plugins/<id>/` → `build-market.mjs` 上架。两条通道打的是同一种包，别混用版本号。

三个可选旗标按语义取用（缺省即公共社区、原创、沿用本地名）：

- `--org <slug>`：发布到组织专用源（仅本人在册组织）。
- `--publish-name <name>`：改名发布（衍生自他人包后的改名上架）。
- `--derived-kind --derived-name [--derived-version]`：声明衍生谱系，必须成对；服务端做存在性、不自指、防环校验。原创发布一律不加。

## 发布前三道校验

先跑脚本拿判定，再决定动作：

```bash
python3 scripts/release_check.py --phase pre --kind skill --name copywriting --version 2.2.0
```

脚本输出 JSON，`verdict` 三档：`pass` 直接发布；`warn` 可发布但要向用户报明风险；`fail` 停下按 `fix` 处置。三项判定含义：

1. **登录态**：`mindx market whoami --json` 的 `has_token` 与 `user.id`。未登录时发布必失败，先让用户在 MindX 账户页登录。
2. **版本占用**：同名包在社区的最新版本与目标版本相同 → 版本字节终身绑定，同版本重发必然 409，必须先把源文件 `version` 调高再发。
3. **引用完整性**：agent 的 `skills`、team 的 `members` 逐条查本机注册表；缺失项打包后仍能上传，但社区端只显示 slug、安装后缺件。

脚本读的版本权威是源文件 frontmatter，不是用户口述的版本号——daemon 会比对两者，不一致直接拒绝发布。改版本就改 frontmatter：

- 技能：`~/.mindx/skills/<name>/SKILL.md` 的 `metadata.version`
- 智能体：`~/.mindx/agents/<name>/IDENTITY.md` 的 `version`
- 团队：`~/.mindx/teams/<name>/TEAM.md` 的 `version`
- 插件：`~/.mindx/plugins/<id>/<激活版本>/manifest.json` 的 `version`（改版本直接改它，指纹由发布端重算）

## 发布

```bash
mindx market publish <agent|skill|team> <name> --version <v> --json
```

- `--json` 必带：回执里的 `sha256` / `created` / `warnings` 是发布后回读的比对基线。
- `warnings` 非空即未通过引用完整性，逐条向用户报明，禁止吞掉。
- 403 包名冲突：加 `--publish-name` 改名，并按被引用的上游加 `--derived-*` 声明谱系。
- 已存在同名包但只想把自己的进化版交给包主：走 `mindx market submit`（投递给 owner 审阅），不是 publish。

## 发布后回读

```bash
python3 scripts/release_check.py --phase post --kind skill --name copywriting --version 2.2.0
```

断言四项：包存在、最新版本等于目标版本、`publisherId` 等于本机账号 UUID、`nameZh`（技能中文名）非空。任一断言失败按 `fix` 处置，禁止口头宣布发布成功。

## 批量增量发布（维护者日常）

单包发布走上面的逐条通道；维护者一次改动几十上百个实体时走批量脚本，
一条命令把「自上次发布以来变更过」的实体整批发出去：

```bash
python3 scripts/publish_batch.py --kind all [--src <目录>] [--dry-run] [--limit N]
```

- `--kind`：`skill` / `agent` / `team` / `all`（all 扫 `~/.mindx` 三类标准根）。
- `--src`：自定义实体根目录（项目里的源目录），目录下每个 `<name>/` 含对应标记文件即可。
- `--dry-run`：只打印清单，不上传、不写台账——首次跑批量必须先 dry-run 核对范围。
- `--no-bump`：内容变但版本未变时不自动升版，转为人工处置。
- `--limit N`：只处理前 N 个，用于小批验证。

判定与处置规则（脚本内固化，不要人工替代）：

1. **变更口径** = 实体目录全部文件内容的指纹（剔除 frontmatter 的 `version` 行）。打包字节带构建时间戳、不可复现，包 sha256 不能用来判断变更。
2. **台账** `~/.mindx/market-publish-state.json`：`<kind>/<name>` → 指纹 + 版本。未变更的实体直接跳过。
3. **安装副本保护**：台账无记录、且本地版本等于远端最新版本 → 判为「从社区装下来的副本」，登记基线后跳过。不加这条会把装下来的展开物当本机改动重发，污染市场元数据。
4. **版本推进**：远端已有同版本 → 自动升 patch 并回写源文件 frontmatter（`--no-bump` 关闭）。
5. **409 兜底**：`market list` 的 version 只是当前主版本，历史版本列表里可能已有同版本 → 服务端回 409。脚本自动再升一次 patch 重试，最多 3 次。
6. **频率限制**：普通账号发布 10 次/小时；维护者账号在服务端白名单（`MARKET_UNLIMITED_PUBLISHERS`）内不受限。撞限流时不要重试轰炸，先确认白名单是否生效。

输出一份 JSON 汇总：`published` / `skipped` / `failed` / `needs_manual`；退出码 1 表示有失败项。

## 纪律边界

- **版本只增不减**：同 `(kind, name, version)` 的字节终身绑定，任何内容改动都要升版本；想「覆盖」已有版本是设计上不存在的操作。
- **发布者身份随包冻结**：`publisher_id` 写入包内清单，新版本才携带；旧版本包没有该字段属既成事实，不要伪造回填。
- **禁止绕开脚本**：登录态、版本占用、引用完整性、发布后回读四项判定一律以脚本输出为准。
- **CLI 输出解析**：stdout 前部混有 `INFO connected to gateway` 日志，取第一个 `{` 之后的内容再解析。

## 失败模式

- 现象：`quota_exceeded`「包数超过上限」。原因：服务端被显式设了 `MARKET_MAX_USER_PACKAGES`（缺省不限）。动作：确认是否有人在 env 里临时限流；不需要就清空该变量并重启服务。
- 现象：`rate_limited`「发布频率超限（10 次/小时）」。原因：批量发布撞单账号限流。动作：维护者账号确认在 `MARKET_UNLIMITED_PUBLISHERS` 白名单内并已重启服务；普通账号等窗口过去或拆批发布。
- 现象：`mindx market publish` 报 409。原因：同版本已发布。动作：升 frontmatter 版本、重载注册表后再发，不改发布参数硬重试。
- 现象：403 包名冲突。原因：社区已有同名包且归属他人。动作：确认是衍生演进 → `--publish-name` + `--derived-*`；不是衍生 → 换名。
- 现象：`尚未登录社区`。原因：daemon 无有效令牌。动作：让用户登录后重跑 pre 校验。
- 现象：发布成功但社区页面看不到中文名。原因：源文件缺 `name_zh` 或 `description`。动作：补 frontmatter 后升版本重发。
- 现象：回读 `publisherId` 为空。原因：目标版本是 `publisher_id` 上线前发布的旧包。动作：如实向用户说明该版本无该字段，需发新版本才携带。
