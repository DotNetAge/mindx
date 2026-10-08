# 插件规格表模板与脚手架映射

轮 4 的确认产物。规格表经用户拍板后，作为 mx-plugin-dev 施工的唯一输入。

## 规格表模板

```markdown
# <插件名> 规格

## 身份
- id：<反向域 id，带插件前缀防撞 id>
- 名称：<显示名>
- 版本起点：1.0.0

## 席位清单
| 区 | 条目 id | order | 内容 | 备注 |
| --- | --- | --- | --- | --- |
| Sidebar 分节 | <前缀>-section | <值> | 节标题 + 行列表 | 行 id 对应 Content 条目 id |
| Content 页 | <前缀>-page | <值> | <一句话> | 行点击到达 |
| Detail tab | <前缀>-detail | — | <一句话> | 由 <哪个页面> 自动打开 |
| Overlay banner | <前缀>-notice | — | <触发时机> | 纯内容组件 |
| Overlay modal | <前缀>-confirm | — | <确认动作> | 危险操作必走 |
| Settings 页 | <前缀>-prefs | — | 页标题 | 行见下 |
| Toolbar | <前缀>-action | — | slot: leading/trailing | |

## 交互流
- <行点击> → <呈现哪个 Content 页> → <是否自动开 Detail>
- <某动作> → <modal 确认> → <banner 反馈结果>
- 停用清理：移除全部席位，配置保留

## 配置项
| 配置 | 归属 Settings 页 | 控件 | 缺省值 |
| --- | --- | --- | --- |

## 服务（跨插件时才填）
- provide：<服务名>（共享的是 Pinia store 响应式本体）
- use：<消费哪些既有服务>

## 脚手架场景
- 场景参数：<basic / --with detail settings overlay services 叠加>
- 生成命令：`python3 .agents/skills/mx-plugin-dev/scripts/scaffold_plugin.py <模块名> <场景参数>`（mindx-work 仓库根目录执行）

## 验收判据
- 断言点：<逐席位一条——分节出现 / 页面呈现 / 详情打开 / 通知弹出 / 配置生效>
- 验证方式：pnpm typecheck + Playwright 断言（market 插件走真机验收链路）
```

## 脚手架场景映射规则（固定，勿自创）

| 规格含 | 场景参数 |
| --- | --- |
| 基础界面（Sidebar / Content，必有） | basic（缺省，不写 --with） |
| Detail 轨道 | `--with detail` |
| Settings 页或行 | `--with settings` |
| Overlay banner 或 modal | `--with overlay` |
| services 提供/消费 | `--with services` |

场景可任意叠加：`scaffold_plugin.py my-plugin --with detail settings overlay services`。骨架自动完成三处注册（exports / re-export / 装配清单）。

## 交接纪律

1. 规格表先给用户过目拍板，确认后才调 mx-plugin-dev。
2. 施工时规格表即任务清单：席位清单逐行注册、验收判据逐条断言，禁止施工时私自加减席位（发现规格缺陷回设计轮修订，不静默偏离）。
3. market 插件的规格另需补发布计划（zip / index.json 登记），读 mx-plugin-publish 技能。
4. 席位清单 order 列守保留范围（contract.md §2）：预置（core）插件落 1–1000 保留段，market 在线（扩展）插件必须从 1001 起。
