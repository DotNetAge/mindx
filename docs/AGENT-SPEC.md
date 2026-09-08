# Agent Bundle 规范（.mindpkg）

本文档定义 Agent / Skill 分发包的容器格式、文件作用，以及导出、本地导入安装、
在线市场安装的完整流程。对应实现：`mindx/internal/core/bundle`（bundle.go / export.go /
install.go / market.go / zip.go），发布脚本 `mindx-app/scripts/publish-market.mjs`。

设计基准（PR-PROMPTS 第四节）：**分发格式 ≠ 存储格式**——"包含"只发生在传输层，
冗余只存在于包内；落地后运行时是纯引用（共享、动态建立与销毁都靠引用列表操作）。

---

## 一、包格式

分发包是单一 zip 容器，扩展名 `.mindpkg`。Skill 包是 Agent 包的子集（同一容器
格式 + 清单，安装器复用同一套展开逻辑）。

```text
<name>.mindpkg（zip 容器）
├── manifest.json                    # 包清单（必需，最后写入）
├── agent/                           # kind=agent 时存在：Agent 目录内容
│   ├── IDENTITY.md                  # Agent 元数据（frontmatter）+ 可选身份正文
│   ├── SOUL.md                      # 行为规则（可缺省）
│   └── skills/<skill-name>/...      # 技能副本：Agent 级技能 + 引用的全局技能副本
└── skills/<skill-name>/...          # kind=skill 时存在：单个技能目录
```

### manifest.json

```json
{
  "format": 1,
  "kind": "agent",
  "name": "architect",
  "description": "专职技术选型、架构设计……",
  "icon": "compass",
  "role": "系统架构师",
  "category": "产品研发",
  "skills": ["research-pipeline", "software-dev"],
  "skill_names": { "flutter-dev": "Flutter应用开发指南" }
}
```

| 字段                   | 作用                                                                                                           |
| ---------------------- | --------------------------------------------------------------------------------------------                   |
| `format`               | 包格式版本（当前唯一版本 `1`，不符即拒装）                                                                     |
| `kind`                 | `agent` / `skill`                                                                                              |
| `name`                 | 包名（Agent 名或技能名，必需）                                                                                 |
| `description` / `icon` | 展示信息（取自 IDENTITY.md frontmatter）                                                                       |
| `role` / `category`    | 岗位头衔 / 业务分类（kind=agent，取自 IDENTITY.md frontmatter，中文；市场卡片标题与分类筛选用，三端同口径）    |
| `skills`               | 包内包含的技能名列表（kind=agent），安装器据此报告落位结果                                                     |
| `skill_names`          | 技能中文展示名映射（kind=agent，技能名 → SKILL.md metadata.name_zh；未声明中文名的技能不收录，展示端回退原名） |

> 历史包兼容：旧包清单中的 `domains` 字段已退役（与 frontmatter 的 `category` 语义
> 重复，统一为单值中文 `category`）；旧包安装后 IDENTITY.md 由加载器自动迁移写回。

### 各文件作用

| 包内路径             | 作用                                                   | 安装去向                    |
| -------------------- | ------------------------------------------------------ | --------------------------- |
| `manifest.json`      | 清单：类型、名称、展示信息、技能清单；校验格式与完整性 | 不落盘                      |
| `agent/IDENTITY.md`  | Agent 唯一 meta 入口（强类型 frontmatter）             | `agents/<name>/IDENTITY.md` |
| `agent/SOUL.md`      | 行为规则（自带标题）                                   | `agents/<name>/SOUL.md`     |
| `agent/skills/<n>/…` | 技能副本（含 Agent 级覆盖版本 + 全局引用副本）         | 按同名规则分流（见下）      |
| `skills/<n>/…`       | 单技能目录（kind=skill 包）                            | `~/.mindx/skills/<n>/`      |

---

## 二、导出（导出保真原则）

导出打包 Agent **实际生效**的技能版本，保证分发包行为与本地运行时一致（"逻辑重写"不漂移）：

```text
ExportAgent(agent, skills, outPath)
  ├─ agent/IDENTITY.md、agent/SOUL.md      # 按磁盘现状打包（保留手写痕迹）
  ├─ agent/skills/<n>/                     # Agent 级技能目录全部内容（实际生效的覆盖版本）
  ├─ agent/skills/<n>/                     # 声明引用但不在 Agent 级目录的全局技能 → 取全局库副本补入
  │     └─ 引用在本地缺失 → 记入 warnings（加载降级语义一致，不阻断导出）
  └─ manifest.json                         # 内容定稿后最后写入
```

- RPC：`agent.export`（返回 .mindpkg 路径）；
- Skill 导出：`skill.export`（全局库单技能 → `skills/<n>/`）。

---

## 三、安装（本地导入 / 在线市场共用一套展开逻辑）

`bundle.Install(pkgPath, opts)`：全量条目先读入内存、完成冲突预检后才写盘
（避免"写一半失败"的中间态）；调用方负责安装后的 `ReloadAgents` / `ReloadSkills`。

### 3.1 Agent 包展开规则

```text
bundle.Install（kind=agent）
  ├─ 冲突预检：agents/<name>/ 已存在 → 默认拒绝
  │     （界面确认后带 Overwrite 重试 → 覆盖）
  ├─ agent/*  ──────────────→  agents/<name>/
  └─ agent/skills/<skill>/
       ├─ 全局库已有同名  ──→  agents/<name>/skills/<skill>/
       │                     # Agent 级覆盖 = 逻辑重写（运行时同名只注册 Agent 内版本）
       └─ 全局库没有      ──→  ~/.mindx/skills/<skill>/
                             # 释放进本地全局技能库，Agent 引用列表天然指向本地
```

**包内 skills 目录的释放规则**（Bundle 中的 `agent/skills/` 只存在于传输层）：
安装后按"全局库是否已有同名"分流——不同名的技能**进入本地全局 Skills 目录**
（成为可共享引用的最佳实践）；同名的技能落 **Agent 目录 skills/**（Agent 级覆盖，
实现逻辑重写）。落地后 Bundle 的包含关系即消除，运行时恢复纯引用。

### 3.2 Skill 包展开规则

`skills/<n>/` 直接进全局库；任一技能与全局库同名 → 默认拒绝，确认后带
`Overwrite` 重试。

**Skill 分类规范**：技能的业务分类放在 SKILL.md 的 `metadata.category`
（中文，与 Agent 的 frontmatter `category` 取同一套业务分类口径）；skillstore 的
Metadata map 原样透传，展示端经 `SkillInfo.metadata.category` 消费。旧技能未声明
该键时分类显示为空，不做枚举映射（本轮仅定规范，技能数据与 SkillManager UI 后续跟进）。

### 3.3 安装结果（RPC 返回前端展示）

`InstallResult{ Kind, Name, SkillsGlobal[], SkillsAgent[], Overwritten }`——
分别报告落入全局库与 Agent 级库的技能名。

---

## 四、COS 静态市场（在线安装）

市场托管于腾讯云 COS（bucket `repo-1257961037`，公有读私有写），以 manifest
清单实现静态货架；sha256 仅用于传输完整性校验，不构成版本管理（同名包内容可更新，
覆盖即发布）。

### 4.1 在线安装流程

```text
market.list（MarketClient.List）
  ├─ GET market/manifest.json                # 货架清单（≤10MB）
  │     ├─ 成功 → 刷新本地缓存（原子替换）→ Source=remote
  │     └─ 失败 → 读本地缓存 → Source=cache + 降级原因提示
  │            （在线与缓存均不可用 → 显式报错，不静默）
  │
market.install（MarketClient.Download → bundle.Install）
  ├─ 缓存命中且 sha256 校验通过 → 复用，不重复下载
  ├─ 下载 → 大小校验 → sha256 校验（不通过拒装，失败不落缓存）
  └─ 校验通过 → bundle.Install（与本地导入同一套展开逻辑）
```

RPC 面：`market.list`（拉货架）、`market.install`（下载+安装，返回 InstallResult）。
前端入口：AgentBrowser（在线列表 / 本地导入 / 导出按钮）、SkillManager（同构）。

### 4.2 manifest.json（市场清单，"货架"）

```json
{
  "version": 1,
  "updated_at": "2026-09-06T00:00:00Z",
  "packages": [
    {
      "kind": "agent",
      "name": "architect",
      "description": "……",
      "icon": "compass",
      "role": "系统架构师",
      "category": "产品研发",
      "file": "packages/architect.mindpkg",
      "sha256": "<64 位十六进制>",
      "size": 12345
    }
  ]
}
```

`file` 为相对清单地址的路径（客户端做路径穿越校验）；清单结构预留扩展字段，
不做超前设计。

### 4.3 发布流程

```text
App 导出（agent.export）→ 产出 .mindpkg
  └─ node scripts/publish-market.mjs [分发包目录]
       ├─ 环境变量 COS_SECRET_ID / COS_SECRET_KEY（必填，桶写权限）
       ├─ 上传包文件到 packages/
       └─ 合并生成 market/manifest.json（默认同 kind+name 覆盖、其余保留；--replace 整体重建）
```

批量打包走市场源仓库（`mindx-market/`）：

```text
mindx-market/
  ├─ agents/<分类>/<name>/     # 岗位 Agent（IDENTITY.md + SOUL.md）
  ├─ skills/<分类>/<name>/SKILL.md   # 市场技能库（按分类归档，库内平铺）
  └─ dist/                     # pack-agents 产物
```

```bash
# 旧单文件 Agent 一次性规范化为目录格式（原地迁移，保留 .bak）
mindx market canonicalize-agents --src ../mindx-market/agents

# 批量打包 + 安装回环验证：--skills 指向平铺技能库（多个库逗号分隔，依序并入），
# Agent 声明引用的全局技能按库副本打入包内（传输层包含，存储纯引用）
mindx market pack-agents --src ../mindx-market/agents \
  --skills ../mindx-market/skills/dev,../mindx-market/skills/marketing \
  --out ../mindx-market/dist
```

设计约定：市场源 Agent 是"薄岗位"——IDENTITY 声明技能装配，SOUL 只写行为准则，
流程一律沉淀为技能（同族共享进全局库，私有进 `<agent>/skills/`）；被岗位合并取代的
旧 Agent 归档至 `mindx-market/archive/`，不进货架。

默认桶 `repo-1257961037`、地域 `ap-guangzhou`、前缀 `market/`（可用
`MARKET_BUCKET` / `MARKET_REGION` / `MARKET_PREFIX` 覆盖）。客户端匿名拉取，
桶必须开「公有读私有写」。

---

## 五、分发闭环验收路径

1. 本地创建 Agent（引用全局技能 + Agent 级覆盖技能）→ `agent.export` 导出 .mindpkg；
2. `publish-market.mjs` 发布 → 市场清单可见；
3. 全新环境 `market.install` 在线安装（或 .mindpkg 本地 `agent.import`）；
4. 验证：Agent 落位 `agents/<name>/`，未引用过的技能进入全局库，同名覆盖版本落
   Agent 级；Agent 可用且技能行为与导出端一致。
