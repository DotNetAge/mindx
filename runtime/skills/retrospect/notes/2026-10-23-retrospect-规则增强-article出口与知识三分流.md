---
version: "1.1.0"
pub_date: 2026-10-23
sha: "10f926343b87d1fcf61ce8a415bcb1fda6f70a1d8349466bc51fb0624fb4988a"
modifier: "Ray"
modifier_id: "1"
---

# 2026-10-23 retrospect 规则增强-article 出口与知识三分流

## 定案

知识体系在 memory 之后增设 article 出口，形成四层双轨：notes（原料）→ memory（提炼）→ skill（行为知识，回答怎么做）| article（认知知识，回答为什么这么想）。

依据：市场 contents/articles 板块已建但从未接通内容管道（0 篇）；复盘素材中大量「讲得清但不构成操作流程」的知识（机制取舍、决策依据、领域复盘）在旧二分流下无去处，只能烂在 memory 或被清。

## 改动

- 步骤 3 候选知识改三分流判定：技能（可写成决策路由）/ 文章（讲得清但不必可执行）/ 插件立项（呈现类缺口），原技能与插件两条出口语义不变
- 新增文章质量门：明确论点 + 真实出处 + 说人话；不要求触发词/决策路由/体积上限（区别于技能门）
- 文稿落 `mindx-market/contents/articles/<slug>/`，发布走 market-content 流程
- 分享前登录三分支路由：`mindx market whoami`（v2.6.0 新增 CLI 薄壳，调既有 `market.account.get` RPC）→ 已绑定直接发布；匿名 → AskUser 引导 work 账户页或 mindx.chat 注册/登录（注册在网页完成，技能不实现注册）；拒绝 → 文章分享不可用，不阻塞复盘

## 关键事实

- `market.account.get` RPC 与客户端封装 `MarketAccountGet()` 早已存在（work 账户页在用），登录态查询零新后端代码
- `/me` 返回的用户名字段是 `nick_name`/`login`/`provider`，不是 `display_name`/`username`
