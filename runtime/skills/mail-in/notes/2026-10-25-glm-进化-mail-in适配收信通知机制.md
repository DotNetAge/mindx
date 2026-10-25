---
version: 1.1.0
pub_date: 2026-10-25
sha: 8dbeecee0a1f616c50a7aab5d320ddb16c92b2e06386f65181e08f80e8fdd983
modifier: Ray
modifier_id: 1
---

# mail-in 进化：适配收信通知机制重构

## 问题

daemon 收信机制重构（信箱负责人机制 + 固定模板通知）：wakeForMail 不再把寄件人、主题、正文渲染进开工内容，只递一句「你有新的企业来信，id 为 XXXXX，请查收」；dead_line 的排期语义整体退役，解释权移交大秘。SKILL.md 正文「daemon 会把寄件人、主题、信件编号与正文直接递给你」的描述与机制不符，且技能缺少组织判读、截止时刻自查、通知兜底三块职责。

## 修正

- 「接到叫醒通知后」：凭编号 `mindx mail inbox` 取信读全文；顺带扫收件箱其他未读信（大秘未雇佣期间堆积的存量信依赖此条补处理，机制不补发通知）
- 新增「判读前的两眼」：mail.members 组织位置 + ~/.mindx/USER.md 用户职责
- 新增「截止时刻是硬约束」：剩余时间评估、Cron 自建提醒、提前改期义务
- 新增「转告主人时把提醒做到位」：TODO.md 落点 + Notify 工具四级紧急度通知（high/medium/low/normal ↔ 红/黄/绿/普通）
- 命令速查补 mail members
- version 1.0.0 → 1.1.0

## 影响与验证

- mindx skill validate 通过；正文日志噪声自查（定案|定稿|用户提出|今天|实测）0 命中
- mail-in 为 runtime 内置技能，不在 mindx-work/.agents/skills 镜像范围（该镜像仅覆盖 mx-* 工作技能集），无需双副本同步
- 识别信号 → 行动：下次看到「daemon 把开工内容整段渲染/排期」类机制描述 → 先核对 wakeForMail 当前实现再写进技能正文，机制描述必须与实现同窗校验
