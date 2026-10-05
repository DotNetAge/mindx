# skill-creator 文档与代码漂移修正记录

日期：2026-10-05
背景：用户指出 skill-creator（技能种子文档）的 references 未关联、schemas.md 扩展字段缺失。修正过程中核实出多处文档与代码不一致，经验如下。

## 事实结论（以 internal/core/skillstore/parse.go 为唯一权威）

- frontmatter 硬性必填只有 `name` 和 `description`；`metadata.version` 非必填，仅打包时写入市场清单（internal/core/bundle/market.go），无版本管理策略
- 依赖未满足（requires.bins/envs/runtime 缺失）**不阻断装载**，只生成 warnings；硬卡点在运行前准备流程（handler_skill_prepare.go，MissingBins 判定）
- env 名单定稿键为 `envs`，兼容存量 `env` 写法；requires 还支持 `runtime`（node/python/dotnet/go/rust/java，约束 `>=N`/`>N`/`^N`/`N.x`/`N`）和 `oauth`（provider+env）
- 与 requires 平级还有 `metadata.install`（平台 → 修复命令块）
- 正文模板变量 `{base_dir}`、`{skill_name}` 加载时自动替换
- name 不规范时自动清洗一次（空格转连字符、转小写），仍不合规才报错

## 教训

1. **文档描述"会被静默跳过"与代码行为（照常装载+告警）相反**——写行为语义必须查代码，不能凭直觉或旧版本印象。
2. **go:embed 目录嵌入自动排除 `.` 和 `_` 开头的条目**——`__pycache__`、`.DS_Store` 等不会进入嵌入 FS，不必当作同步污染处理，但仍是应清理的死文件。embed.go 清单是目录级（`//go:embed runtime/skills`），目录内增删普通文件无需改清单。
3. **runtime/skills/skill-creator/references/ 下 4 个文档（spec/best-practices/optimizing-descriptions/using-scripts.md）一直是 untracked 状态**——目录级 embed 按磁盘内容嵌入不影响本机构建，但换机器 clone 后会缺失，需要 git add 提交。

## 本次改动

- references/schemas.md：补全 metadata 扩展键定义（requires 详表/install/version/name_zh 用途）、修正装载语义、新增正文模板变量小节、示例补 version/requires
- references/spec.md：文首加 MindX 差异说明（license/compatibility 解析后丢弃）
- SKILL.md：description 扩展审查/优化触发、新增"参考文档索引"与"审查已有技能"流程（步骤 A-D）、version 升 1.1.0
- 删除死目录 scripts/（仅剩 __pycache__/*.pyc，源文件已不存在）与空 assets/
- 验证：mindx skill add + validate 通过，metadata（version 1.1.0）正确展示
