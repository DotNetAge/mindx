# PR: Agent分类与上线支持

## 需求

- 要实现Agent的分类浏览功能，根据Agent的`Meta["domains"]`字段，将Agent分类到不同的领域，这个功能是数据层面与界面操作层面需要的。
- 在 Agent中增设`Meta["hired"]`字段，用于记录Agent是否已被用户雇佣，只有被雇佣的Agent才对会话可用。
  - 此功能将会影响到 Agent 的查询指令，指令要增加 `--all` 参数来筛选全部Agent，默认情况下只查询已雇佣的Agent。
- 这个功能的增设可能会影响到 MindX-App的 Agent列表，Agent管理器，需要进行检查。
- MindX-App要增加一个AgentBrowser，用于显示全部的Agent，作用如下：
  - 根据Domain标签查看该领域的Agent列表
  - 根据 roles, name 筛选 Agent
  - 每个Agent项显示Agent的名称、角色、描述、是否被雇佣信息；

## 已定设计决策

1. **过滤层采用方案 A：注册表全量 + 雇佣视图**
   - `AgentRegistry` 保持全量加载（目录即注册），不改变注册表本身的注册语义；
   - `App` 新增雇佣视图（如 `HiredAgents()`），所有**会话可用入口**（TUI `/agent` 选择器、`resolveCurrentAgentName` 回退、App 侧 Agent 切换）只走雇佣视图；
   - 所有**管理与浏览入口**（`agent.get/update/score`、AgentBrowser、CLI `agent list`）走全量，保证未雇佣的 Agent 可见、可雇佣、可打分。
   - 理由：避免"未注册则够不着雇佣操作"的鸡生蛋问题；`agent.score` 也可对未雇佣 Agent 打分，辅助浏览决策。
2. **`Meta["hired"]` 缺省为 `false`**（未雇佣）。第一批预置 Agent 由手动修改补充 hired/domains 数据。
   - **goharness 框架层零改动**：`AgentConfig.Meta map[string]any` 已存在的扩展口即可承载业务键，框架不解释其语义；
   - `hired`/`domains` 的取值 helper（`IsHired() bool` / `Domains() []string`，容忍 bool 与字符串 `"true"` 等写法）作为 **mindx 应用层**函数实现（`internal/core` 独立 helper 文件），业务语义不下沉框架。
3. **RPC `agent.list` 保持全量返回**（meta 随行），向后兼容 mindx-app 的 `fetchAgents` 与 CLI `--json`；hired/domains 过滤放在客户端展示层（CLI 默认隐藏未雇佣，`--all` 显示全部）。
4. **雇佣/解雇走专用 RPC `agent.hire` / `agent.fire`**
   - 避开 `agent.update` 对 `Meta` 的整包替换语义（`p.Meta != nil` 直接覆盖 `updated.Meta`），防止只想改 hired 时把 domains 等其他 meta 键抹掉；
   - CLI 对应增加 `mindx agent hire <name>` / `mindx agent fire <name>`。
5. **domains 为 `[]string` 多值自由标签**
   - YAML frontmatter 写法：`meta: { domains: [coding, writing] }`；存储统一小写规范化；
   - 不建预定义领域枚举表；AgentBrowser 的 Domain 标签栏由 `agent.list` 结果聚合去重生成。

## 影响面检查清单

| 位置                                                               | 处理                                                                                          |
| ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------- |
| TUI `/agent` 浮层选择器（internal/client/registry.go）             | 改走雇佣视图，只显示已雇佣 Agent                                                              |
| `resolveCurrentAgentName` 回退 `List()[0]`（internal/core/app.go） | 基于雇佣视图取值；雇佣视图为空时给出明确引导                                                  |
| CLI `agent add`                                                    | 新 Agent 不写 hired 字段即默认未雇佣，创建后需 `agent hire` 显式雇佣（与缺省 false 语义一致） |
| `agent score`                                                      | 注册表全量下无影响，可对未雇佣 Agent 打分                                                     |
| mindx-app AgentConfig 管理页                                       | 重新定位为「我的团队」：只显示已雇佣 Agent，继续承担编辑管理                                  |
| mindx-app AgentBrowser（新增）                                     | 「人才市场」：显示全部 Agent + Domain 分类 + roles/name 筛选 + 一键雇佣/解雇                  |

### CLI 指令影响明细（cmd/）

**需要改动：**

| 指令                            | 处理                                                                                                                                                                                |
| ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `agent list`                    | 加 `--all` 显示全部；默认只显示已雇佣。本地加载路径（LoadAgentsFrom 后用应用层 IsHired 过滤）与 `--json` daemon 路径（客户端过滤）两条路径都要实现；可顺带加 `--domain <标签>` 过滤 |
| `session create --agent <name>` | daemon 侧 `handleSessionCreate` 当前连 agent 存在性都不校验，直接建会话；需补「必须已雇佣」校验（先查存在、再查 IsHired），未雇佣时报错并引导 `mindx agent hire`                    |
| `schedule add --agent <name>`   | 同上：定时任务绑定的 agent 必须已雇佣，防止未雇佣 Agent 被 cron 拉起干活                                                                                                            |
| `agent hire/fire`（新增）       | 走专用 RPC，输出变更确认                                                                                                                                                            |

**确认不受影响：**

| 指令                                         | 理由                                                                                |
| -------------------------------------------- | ----------------------------------------------------------------------------------- |
| `agent get` / `agent score` / `agent update` | daemon 走全量注册表，管理语义；浏览/打分/更新未雇佣 Agent 是合法操作                |
| `agent rm`                                   | 删除与雇佣无关，本地全量加载后删除                                                  |
| `agent add`                                  | 创建后默认未雇佣；建议创建输出提示「尚未雇佣，使用 `mindx agent hire <name>` 启用」 |
| `reload agents`                              | 热重载全量注册表，管理语义                                                          |
| `session list --agent`                       | 历史会话过滤，与雇佣状态无关                                                        |
| `query`                                      | `AgentName: "_shared"` 仅为 memory 命名空间，非 Agent 选择                          |
| `install` / root 向导                        | 仅创建 agents 目录并预置数据，预置 Agent 默认未雇佣（符合缺省 false 设计）          |

## 落地切分

1. **数据层（仅 mindx）**：应用层 `IsHired()` / `Domains()` helper + `App.HiredAgents()` 雇佣视图 + `agent.hire` / `agent.fire` RPC + CLI `agent hire/fire`、`agent list --all`（最小闭环，不碰 UI，不动 goharness）
2. **App 层**：AgentBrowser（全量列表 + domain 聚合 + roles/name 筛选 + 雇佣按钮）→ AgentConfig 收窄为已雇佣管理
3. **TUI 层**：`/agent` 选择器接入雇佣视图
