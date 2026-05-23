# 一个人的公司：用 AI Agent 运转整个业务

## 洞察

我见过的每个 AI Agent 平台都把 Agent 当作**执行者**：你给一个任务，它做完，结束。即使是那些号称"多 Agent 协作"的系统，本质上也只是把一个大任务拆成子任务、分派出去、收集结果的花哨方式。

但公司不是这样运作的。

一家公司有：
- **员工**——不同角色、持续负责不同工作
- **日程**——每天、每周、每月重复的事务
- **会议**——人们互相沟通、检查进度、给反馈
- **管理者**——制定计划、跟踪进展、调整方向、汇报结果
- **上下文**——对话跨天跨周，不会清空重来

如果用 AI Agent 来构建这家公司，需要写多少代码？答案是：**几乎不需要**。因为所有基础设施都已经存在，只是没人把它们按这个方式组合过。

## 积木

我们已经有全部零件，只是没看出这个模式。

### 1. Agent（Reactor 循环）

每个 Agent 运行一个"思考-行动-观察"循环。收到消息，思考，决策，执行工具，观察结果，回复。这是标准的 LLM Agent 模式——没有任何新东西。

### 2. Session（会话）

Session 是对话历史存活的地方。Agent 回复了一条消息，这条交换就被保存了。下次用同一个 session ID，Agent 从上次停下的地方继续。

同样是标准功能。每个聊天系统都有。

### 3. Scheduler（调度器）

Scheduler 的工作极其简单：在某个 cron 时间点，向某个 Agent 的某个 session 发一条消息。就这些。Scheduler 不知道自己在"管理项目"或"运转业务"。它只是按时发消息。

### 4. AgentTalk——唯一缺失的零件

（这是我们唯一需要构建的东西。）

Agent 之间可以互相说话。不是通过工作流引擎，不是通过 DAG——就是一个工具调用：

```
AgentTalk(agent_name="@writer", session_id="proj-42", message="报告写得怎么样了？")
```

目标 Agent 被唤醒，看到消息，回复。调用方拿到回复。同一个 session 下次再用，上下文连续。

## 组合——公司是如何涌现的

这些积木没有一个知道全局。每个都在做微不足道的事。但组合在一起，它们产生了类似公司的行为。

### 第一步：管理者制定计划

用户告诉项目经理 Agent："我想运营一个小红书账号。"

PM 跟用户沟通，提取可量化的目标，分解任务：

```
项目: 小红书运营
目标: 3 个月内从 0 涨粉到 10000
任务:
  - @writer: 每周写 3 篇文案（周一/三/五 10:00）
  - @designer: 每周做 3 张配图（周一/三/五 9:00）
  - @analyst: 每周出数据报告（周六 18:00）
```

每个任务都注册到 Scheduler：谁、什么时候、做什么。

### 第二步：Scheduler 叫醒大家

周一早上 9 点。Scheduler 给 @designer 发了一条消息：

> "这周要做 3 张配图。主题：AI 效率工具。"

@designer 不知道自己是"被调度"的。它只看到一条消息，干活，出图。

但任务的 prompt 里有一句额外的指示：

> "完成后，用 AgentTalk 向 project-manager 汇报结果，session 为 'little-red-book'。"

### 第三步：Agent 主动汇报

完成后，@designer 调用：

```
AgentTalk("project-manager", "little-red-book", "3 张配图已做完。主题：AI 写作、AI 编程、AI 设计。")
```

PM 收到汇报，确认，继续。

周一 10 点。@writer 写完文案，同样汇报回来。

### 第四步：管理者跟踪和调整

一周下来，PM 收到了所有 Agent 的汇报。它整理成周报，主动推送给用户——不等用户来问。

如果 @writer 汇报说"选题枯竭"，PM 可以直接回复：

```
AgentTalk("@writer", "little-red-book", "试试写你用那款新 AI 设计工具的体验。个人故事比纯干货效果好。")
```

不需要轮询，不需要仪表盘。就是对话。

### 结果

从用户视角看：他们有一个模糊的想法（"运营一个社交媒体账号"），现在有一队 Agent 在自主工作、主动汇报，用户每天收到简报就行。

从系统视角看：没有什么特别的事发生。一个调度器发了消息。Agent 回复了。Session 存了文本。一个工具在 Agent 之间路由了消息。

魔力不在任何一个组件。魔力在**组合方式**。

## 为什么没人这样做过

大多数 AI 平台用**计算思维**看待多 Agent：我如何把一个大任务分布到多个 LLM 调用上？结果是工作流 DAG、任务队列、编排流水线。

这个方案用的是**管理思维**：我如何运转一个 Agent 有持续职责的组织？

区别微妙但深刻：

| | 计算思维 | 管理思维 |
|---|---|---|
| 工作单元 | 任务 | **持续职责** |
| Agent 生命周期 | 随任务创建，随任务销毁 | **持久存在**，响应消息 |
| 通信方式 | 工作流传递数据 | **对话**，通过共享 session |
| 协调方式 | 编排器控制的 DAG | **自主执行**，管理者引导 |
| 状态 | 不可变工作流状态 | **会话历史**，累积上下文 |

Session 是关键洞察。在计算系统中，状态是一个需要管理的问题。在管理系统中，**对话历史就是状态本身**——每个 LLM 平台都已经有了。

## 代码

整个 AgentTalk 实现大约 100 行 Go：

```go
type AgentTalkFunc func(ctx context.Context, to, sessionID, message string) (string, error)

type AgentTalkTool struct {
    talk AgentTalkFunc
}

func (t *AgentTalkTool) Execute(ctx context.Context, params map[string]any) (any, error) {
    to := params["agent_name"].(string)
    sessionID := params["session_id"].(string)
    message := params["message"].(string)
    
    reply, err := t.talk(ctx, to, sessionID, message)
    return map[string]any{
        "reply":      reply,
        "agent_name": to,
        "session_id": sessionID,
    }, nil
}
```

默认实现克隆调用方的 config，通过 Agent.Ask() 路由——跟每条用户消息走的是同一条路径。不需要任何新基础设施。

## 这能做什么

- **一个人的公司（OPC）**：一个人类 + 一个 PM Agent + N 个专业 Agent = 一整个组织
- **自主运营**：PM 只需计划一次，Agent 无限期地执行和汇报
- **自然协作**：Agent 之间像同事一样对话，而不是像微服务一样传数据
- **零仪表盘**：PM 告诉你发生了什么。你永远不需要去查。

代码开源在 [github.com/DotNetAge/mindx](https://github.com/DotNetAge/mindx)。AgentTalk 工具在 goreact 框架中：[github.com/DotNetAge/goreact](https://github.com/DotNetAge/goreact)。
