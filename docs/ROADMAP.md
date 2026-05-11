# Agent2.0 的能力维度

Agent已不单纯是一个“电脑副驾驶”，而真正衍生为一个“能思考，能进化，能动手”的电子化员工。

- 自主性 Autonomy 
  - 元认知（防呆机制）
  - 记忆体（企业数字资产）
- 工具调用 Tool Usage
  - 工具是Agent的天生能力（是天性）
- 适应性 Adaptability
  - 自组装
  - 自进化
- 决策力 Decision Making
- 编排能力 Orchestration
  - 工具即编排（Tool Orchestration）- 由大模型自然理解自然编排；
  - 职能即编排（Function Orchestration）- 专业的事情只由专家处理；
- 经验策略 Skill Strategy
  - 下一代软件的交付形态：Skills + Scripts
  - 技能是Agent的后天能力，可以学习进化（是后天性）

<svg width="400" height="400" viewBox="0 0 400 400" xmlns="http://www.w3.org/2000/svg">
  <!-- 背景与标题 -->
  <rect width="400" height="400" fill="white"/>
  <text x="200" y="30" font-family="Arial" font-size="18" text-anchor="middle" font-weight="bold">Agent的能力维度</text>

  <!-- 雷达图中心点 -->
  <circle cx="200" cy="200" r="4" fill="#333"/>

  <!-- 6 个维度标签（角度均匀分布） -->
  <g font-family="Arial" font-size="14" fill="#333" text-anchor="middle">
    <text x="200" y="60">自主性</text>
    <text x="325" y="125">工具调用</text>
    <text x="325" y="275">适应性</text>
    <text x="200" y="340">决策力</text>
    <text x="75" y="275">编排能力</text>
    <text x="75" y="125">经验策略</text>
  </g>

  <!-- 雷达网格线（3层同心圆 + 6条辐射轴） -->
  <g stroke="#ccc" stroke-width="1" fill="none">
    <circle cx="200" cy="200" r="60"/>
    <circle cx="200" cy="200" r="120"/>
    <circle cx="200" cy="200" r="180"/>
    <line x1="200" y1="200" x2="200" y2="20"/>
    <line x1="200" y1="200" x2="340" y2="110"/>
    <line x1="200" y1="200" x2="340" y2="290"/>
    <line x1="200" y1="200" x2="200" y2="380"/>
    <line x1="200" y1="200" x2="60" y2="290"/>
    <line x1="200" y1="200" x2="60" y2="110"/>
  </g>

  <!-- 能力覆盖区域（6维度占比相同，统一 80%） -->
  <polygon 
    points="200,80 312,134 312,266 200,320 88,266 88,134" 
    fill="#4292c6" fill-opacity="0.3" stroke="#2979ff" stroke-width="2"/>
</svg>



站在软件视角，Agent2.0不是一个软件，而是一个软件平台与运行环境，更准确地说Agent软件正在转变为 AgentOS。

- Tools = cli tools;
- Skills + Scripts = Software;
- Agent = Kernel;
- TUI / WebUI = Shell;
- Memory = Database;