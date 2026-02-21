# MindX 系统架构

> 一个具备思考能力且可自主进化的 AI 智能助手

---

## 一、整体架构（整洁架构 Clean Architecture）

```mermaid
flowchart TB
    subgraph "表现层 / 适配器层 Adapters"
        direction TB
        A1[Web Dashboard<br/>React + Vite]
        A2[CLI 命令行<br/>Cobra]
        A3[TUI 终端界面<br/>Bubble Tea]
        A4[消息网关 Gateway]
        A5[HTTP API 接口<br/>Gin]
    end
    
    subgraph "应用层 / 用例层 Use Cases"
        direction TB
        B1[Brain 仿生大脑]
        B2[Memory 记忆系统]
        B3[Skills 技能管理]
        B4[Capability 能力管理]
        B5[Session 会话管理]
        B6[Cron 定时任务]
        B7[Embedding 向量化服务]
        B8[Training 自训练模块]
    end
    
    subgraph "核心层 / 实体层 Core / Entities"
        direction TB
        C1[Thinking 思考接口]
        C2[Brain 大脑接口]
        C3[Memory 记忆接口]
        C4[Assistant 助手接口]
        C5[Channel 通道接口]
        C6[实体定义<br/>Capability/Session/Skill]
    end
    
    subgraph "基础设施层 Infrastructure"
        direction TB
        D1[Bootstrap 启动引导]
        D2[Persistence 持久化<br/>Badger KV / SQLite]
        D3[Embedding Provider<br/>Ollama / TF-IDF]
        D4[LLM Provider<br/>Ollama / OpenAI API]
        D5[Cron Scheduler<br/>Crontab / Windows Task]
        D6[Logging 日志系统<br/>Zap]
    end
    
    subgraph "外部系统 External Systems"
        direction TB
        E1[Ollama 本地模型]
        E2[云端大模型<br/>GLM / Qwen / Claude]
        E3[社交平台<br/>微信/钉钉/QQ/飞书]
        E4[Telegram / WhatsApp]
        E5[iMessage / Facebook]
    end

    A1 --> A5
    A2 --> B1
    A3 --> B1
    A4 --> B1
    A5 --> B1
    
    B1 --> C1
    B1 --> C2
    B2 --> C3
    B3 --> C6
    B4 --> C6
    B5 --> C6
    B6 --> D5
    B7 --> D3
    B8 --> E1
    
    C1 --> D4
    C3 --> D2
    C5 --> E3
    C5 --> E4
    C5 --> E5
    
    D3 --> E1
    D4 --> E1
    D4 --> E2
```

---

## 二、仿生大脑架构

```mermaid
flowchart LR
    subgraph "Bionic Brain 仿生大脑"
        direction TB
        
        subgraph "Subconscious 潜意识层<br/>(本地微型模型)"
            LB[左脑 Left Brain<br/>- 意图识别<br/>- 关键词提取<br/>- 简单问答<br/>模型: qwen3:0.6b]
            RB[右脑 Right Brain<br/>- 技能调用<br/>- Function Call<br/>- 工具执行]
        end
        
        subgraph "Consciousness 主意识层<br/>(云端/能力模型)"
            C[意识 Consciousness<br/>- 深度思考<br/>- 复杂推理<br/>- 能力调用]
        end
        
        subgraph "Memory System 记忆系统"
            STM[短期记忆<br/>Session History]
            LTM[长期记忆<br/>Vector Store]
            PM[永久记忆<br/>Fine-tuned Model]
        end
    end
    
    Input[用户输入] --> LB
    
    LB -->|可以回答?| Output1[直接回答]
    LB -->|需要技能?| RB
    LB -->|无法回答?| C
    
    RB --> Output2[执行技能返回结果]
    C --> Output3[深度思考返回结果]
    
    LB <--> Memory_System
    RB <--> Memory_System
    C <--> Memory_System
    
    Memory_System <-->|记忆提取| LB
    Memory_System <-->|记忆提取| RB
    Memory_System <-->|记忆提取| C
    
    Memory_System <-->|记忆沉淀| LB
```

---

## 三、消息处理流程

```mermaid
sequenceDiagram
    participant User as 用户
    participant Channel as 消息通道
    participant Gateway as 网关 Gateway
    participant Brain as 仿生大脑
    participant Memory as 记忆系统
    participant Skills as 技能管理器
    participant CapMgr as 能力管理器
    participant Model as 大模型
    
    User->>Channel: 发送消息
    Channel->>Gateway: 转发消息
    Gateway->>Brain: 处理请求
    
    Brain->>Memory: 获取相关记忆
    Memory-->>Brain: 返回记忆片段
    
    Brain->>Brain: 左脑思考<br/>(意图识别/关键词提取)
    
    alt 可以直接回答
        Brain->>Model: 本地微型模型
        Model-->>Brain: 返回答案
    else 需要技能
        Brain->>Skills: 搜索匹配技能
        Skills-->>Brain: 返回工具 Schema
        Brain->>Brain: 右脑处理<br/>(Function Call)
        Brain->>Skills: 执行技能
        Skills-->>Brain: 返回技能结果
    else 需要深度思考
        Brain->>CapMgr: 匹配能力
        CapMgr-->>Brain: 返回能力配置
        Brain->>Brain: 主意识激活
        Brain->>Model: 云端大模型
        Model-->>Brain: 返回深度思考结果
    end
    
    Brain->>Memory: 沉淀新记忆
    Brain-->>Gateway: 返回回答
    Gateway-->>Channel: 转发回答
    Channel-->>User: 显示回答
```

---

## 四、目录结构与模块关系

```mermaid
flowchart TB
    subgraph "Root 根目录"
        cmd[cmd/main.go<br/>程序入口]
        config[config/<br/>配置文件 YAML]
        dashboard[dashboard/<br/>React 前端]
        internal[internal/<br/>核心业务]
        pkg[pkg/<br/>公共包]
        skills[skills/<br/>技能目录]
    end
    
    subgraph "internal 核心业务"
        adapters[adapters/<br/>适配器层]
        core[core/<br/>核心层]
        entity[entity/<br/>实体定义]
        usecase[usecase/<br/>用例层]
        infrastructure[infrastructure/<br/>基础设施]
    end
    
    subgraph "adapters 适配器"
        channels[channels/<br/>- 钉钉/微信/QQ<br/>- 飞书/Telegram<br/>- WhatsApp/iMessage]
        cli[cli/<br/>命令行工具]
        http[http/handlers/<br/>HTTP API]
    end
    
    subgraph "usecase 用例"
        brain[brain/<br/>仿生大脑实现]
        memory[memory/<br/>记忆系统]
        skills_uc[skills/<br/>技能管理]
        capability[capability/<br/>能力管理]
        session[session/<br/>会话管理]
        cron_uc[cron/<br/>定时任务]
        training[training/<br/>自训练]
    end
    
    subgraph "infrastructure 基础设施"
        bootstrap[bootstrap/<br/>启动引导]
        persistence[persistence/<br/>- Badger KV<br/>- SQLite]
        embedding[embedding/<br/>- Ollama<br/>- TF-IDF]
        llama[llama/<br/>Ollama 集成]
    end
    
    cmd --> bootstrap
    bootstrap --> brain
    bootstrap --> memory
    bootstrap --> skills_uc
    bootstrap --> capability
    bootstrap --> session
    bootstrap --> cron_uc
    
    brain --> core
    memory --> core
    skills_uc --> core
    capability --> core
    
    channels --> brain
    http --> brain
    cli --> brain
    
    persistence --> memory
    embedding --> skills_uc
    llama --> brain
```

---

## 📋 架构关键特性说明

| 层级/组件 | 颜色 | 说明 |
|-----------|------|------|
| 表现层 | 🔵 | Web、CLI、TUI、多渠道消息接入 |
| 应用层 | 🟢 | 仿生大脑、记忆、技能、能力、会话管理 |
| 核心层 | 🟡 | 接口定义、实体、业务规则 |
| 基础设施层 | 🔴 | 持久化、模型集成、日志、调度 |
| 左脑 | 🟣 | 本地微型模型，快速处理简单任务 |
| 右脑 | 🔴 | 技能调用、Function Call 执行 |
| 主意识 | 🔵 | 深度思考、复杂推理、云端模型 |
| 记忆系统 | 🟢 | 短期/长期/永久记忆三层结构 |

---

## 🛠 技术栈

| 类别 | 技术 |
|-----|-----|
| 后端 | Go 1.25+、Gin、Cobra、Bubble Tea |
| 前端 | React、Vite、Tailwind CSS |
| 数据库 | Badger KV、SQLite |
| 模型 | Ollama、OpenAI API、GLM、Qwen |
| 日志 | Zap、Lumberjack |
| 配置 | Viper、YAML |

---

## 📦 项目核心组件说明

### 1. 整洁架构四层设计

- **表现层/适配器层**：负责外部交互，包括Web界面、命令行、终端界面、多渠道消息接入、HTTP API
- **应用层/用例层**：包含核心业务逻辑，包括仿生大脑、记忆系统、技能管理、能力管理、会话管理、定时任务、向量化服务、自训练模块
- **核心层/实体层**：定义核心接口和业务实体，包括思考接口、大脑接口、记忆接口、助手接口、通道接口
- **基础设施层**：提供技术支持，包括启动引导、持久化、模型集成、日志系统、调度器

### 2. 仿生大脑三层结构

- **左脑**：使用本地微型模型（如 qwen3:0.6b），负责意图识别、关键词提取、简单问答
- **右脑**：负责技能调用、Function Call 执行
- **主意识**：深度思考、复杂推理，使用云端大模型
- **记忆系统**：短期记忆、长期记忆、永久记忆三层结构

### 3. 支持的社交渠道

- 钉钉、微信、QQ、飞书、WhatsApp、Telegram、iMessage、Facebook 等
