---
version: "1.4.1"
pub_date: 2026-11-19
sha: "3128f0bfc8529464950186300551ee350b7c311436c67c2480422a86ec8acc5e"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-19 新建智能体没有头像，卡片只能显示首字占位（mainagent）

问题：流程漏洞——创建流程只覆盖 IDENTITY.md/SOUL.md 的字段与正文，头像（icon）没有任何一步，新建实体在成员卡片上恒为空图占位；市场侧虽有头像库存，但 Agent 无从得知该挑哪张、如何编码。
否决：让 LLM 直接读图再手写 base64——易超上下文、易编错，且无法判断库存图是否已被别的实体占用，重复派图无法发现。
修正：步骤 6「补全定义文件」新增「头像」一条，统一调用 `~/.mindx/assets/bin/pick-avatar.py --group agent`：脚本按 128px 灰度指纹判占用（MSE<100 判同图），写回单行 data URI。version 1.4.0 → 1.4.1。
验证：`python3 ~/.mindx/assets/bin/pick-avatar.py --group agent --target /tmp/avatar-test/IDENTITY.md --dry-run` 出计划、去掉 --dry-run 写回后 `icon_b64()` 可读回 19200 字符；`mindx skill validate --path runtime/skills/agent-creator` 无 issues。