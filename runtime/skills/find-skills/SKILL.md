---
name: find-skills
description: 发现与加载技能：先用 mindx cli 查本地技能库（项目级 .agents/skills 与已注入的内置技能），未命中再查 mindx 市场技能库（market list/install）。当用户问"如何做某事"、"找一个能做 X 的技能"、"有没有能...的技能"，或表达扩展能力的兴趣时使用。
---

# 查找技能

两层查找，顺序固定：**先本地，后市场**。本地命中即加载，禁止跳过本地直接搜市场。

## 何时使用

用户出现以下情况时使用：

- 问"如何做 X"，而 X 可能是有现成技能的常见任务
- 说"找一个能做 X 的技能"或"有没有能做 X 的技能"
- 问"你能做 X 吗"，而 X 需要专业能力
- 想扩展智能体的能力、搜索工具/模板/工作流
- 提到需要特定领域的帮助（设计、测试、部署等）

## 第一步：查本地技能库（mindx cli）

mindx 的本地技能库分两层：

| 层             | 位置                                    | 加载方式                                 |
| -------------- | --------------------------------------- | ---------------------------------------- |
| 用户级（内置） | `~/.mindx/skills/`（随 mindx 安装释放） | 描述已注入当前会话，直接按名判断是否命中 |
| 项目级（动态） | `<projectDir>/.agents/skills/`          | 不注入系统提示，需先列举再按名加载       |

项目级动态技能用 cli 列举：

```bash
mindx skills discovery --json        # 当前项目，结构化输出（name/description/root_dir）
mindx skills discovery --dir <路径>   # 指定项目目录
mindx skills discovery               # 表格输出
```

- 在 discovery 输出中按需求匹配 `description`（关键词优先匹配领域词与动作词）；
- **命中**：用 Skill 工具按技能名加载并遵循其流程，禁止再自行重读 SKILL.md 后另建流程；
- **未命中**：进入第二步。禁止在未查本地前就发起市场搜索。

## 第二步：查市场技能库（本地未命中时）

mindx 内置市场（需 daemon 运行，即已执行 `mindx start`）：

```bash
mindx market list --kind skill                          # 列出全部技能包
mindx market list --kind skill --filter <关键词> --json  # 按关键词过滤（匹配名称/描述/类别，结构化输出）
```

- 在市场清单中按 `name`/`description`/`category` 匹配用户需求；
- 输出含安装量类元信息时优先推荐更成熟的包；无元信息时向用户如实呈现候选，由用户选择。

## 第三步：安装与生效

- 安装市场技能包（sha256 校验，落位全局技能库 `~/.mindx/skills/`）：

```bash
mindx market install skill <name>          # --overwrite 覆盖同名技能
```

- 安装结果由 daemon 落位，若当前会话未立即识别到新技能，执行 `mindx reload skills` 热重载；
- 市场同时分发 agent 包（落 `~/.mindx/agents/`，默认未雇佣，需 `mindx agent hire <name>` 后才能进会话）；
- 项目级技能不走市场：直接创建 `<projectDir>/.agents/skills/<name>/SKILL.md` 即可，动态技能按需读盘加载，无需 reload。

## 仍找不到时

如实告知未找到，不编造技能名：

1. 用通用能力直接帮用户完成任务；
2. 任务跨会话高频复用时，提议沉淀为项目技能（`<projectDir>/.agents/skills/<name>/SKILL.md`，frontmatter：name/version/description），**写入前必须经用户授权**。
