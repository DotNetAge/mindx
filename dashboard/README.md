# MindX Dashboard

MindX Dashboard 是一个基于 React 和 TDesign 构建的 Web 界面，用于与 MindX 系统进行交互。

## 技术栈

- **React 18** - 前端框架
- **TypeScript** - 类型安全
- **Vite** - 构建工具
- **TDesign React** - 企业级 UI 组件库
- **Tailwind CSS** - 原子化 CSS 框架

## 功能特性

### 1. 对话界面
- 实时流式对话
- 代码高亮显示
- 支持 Markdown 格式
- 技能调用状态显示

### 2. 历史记录
- 对话历史管理
- 搜索过滤
- 快速加载历史对话

### 3. 设置面板
- 模型配置（Ollama、OpenAI、Anthropic）
- 技能管理
- 通用设置（主题、语言、通知）

## 开发指南

### 安装依赖

```bash
cd dashboard
npm install
```

### 开发模式

```bash
npm run dev
```

访问 http://localhost:5173

### 生产构建

```bash
npm run build
```

构建产物在 `dist/` 目录

### 预览构建

```bash
npm run preview
```

## 使用方法

### 启动 Dashboard

从项目根目录运行：

```bash
mindx dashboard
```

这会：
1. 启动后端 API 服务器（端口 911）
2. 自动打开浏览器访问 http://localhost:911

### 手动启动后端

```bash
go run cmd/dashboard/main.go
```

访问 http://localhost:911

## 项目结构

```
dashboard/
├── src/
│   ├── components/       # React 组件
│   │   ├── Chat.tsx          # 对话主界面
│   │   ├── MessageList.tsx    # 消息列表
│   │   ├── MessageInput.tsx   # 消息输入
│   │   ├── Sidebar.tsx        # 侧边栏导航
│   │   ├── History.tsx        # 历史记录
│   │   └── Settings.tsx      # 设置面板
│   ├── styles/          # 组件样式
│   ├── App.tsx          # 主应用
│   ├── main.tsx         # 入口文件
│   └── index.css       # 全局样式
├── public/             # 静态资源
├── dist/              # 构建产物
├── index.html         # HTML 模板
├── vite.config.ts     # Vite 配置
├── tailwind.config.js # Tailwind 配置
└── package.json       # 项目配置
```

## 设计规范

### 配色方案

OpenClaw 风格深色主题：

- 背景色：`#030712` (gray-950)
- 容器背景：`#111827` (gray-900)
- 文字主色：`#f9fafb` (gray-50)
- 文字次色：`#9ca3af` (gray-400)
- 边框色：`#374151` (gray-700)
- 主色调（蓝色）：`#3b82f6`
- 成功色（绿色）：`#10b981`
- 错误色（红色）：`#ef4444`

### 组件规范

1. 所有组件使用 TypeScript 编写
2. 样式优先使用 Tailwind CSS
3. 使用 TDesign 组件库提供的组件
4. 图标使用 TDesign Icons 或 Lucide React
5. 遵循单一职责原则，每个组件不超过 300 行

## API 接口

### POST /api/chat

流式对话接口

**请求体：**
```json
{
  "message": "用户消息",
  "history": [
    {
      "id": "1",
      "role": "user",
      "content": "历史消息",
      "timestamp": 1234567890
    }
  ]
}
```

**响应：** Server-Sent Events (SSE) 流

### GET /api/skills

获取所有可用技能

**响应：**
```json
[
  {
    "name": "screenshot",
    "description": "屏幕截图",
    "type": "cli"
  }
]
```

### GET /health

健康检查

**响应：**
```json
{
  "status": "ok"
}
```

## 开发注意事项

1. 每次修改前端代码后需要重新构建：
   ```bash
   cd dashboard && npm run build
   ```

2. 后端会自动从 `dashboard/dist` 目录提供静态文件

3. 跨域已启用，支持本地开发

4. 默认端口为 8080，可在 `cmd/dashboard/main.go` 中修改

## 待实现功能

- [ ] 用户认证
- [ ] 会话持久化
- [ ] 文件上传
- [ ] 技能测试界面
- [ ] 日志查看器
- [ ] 实时监控仪表板
