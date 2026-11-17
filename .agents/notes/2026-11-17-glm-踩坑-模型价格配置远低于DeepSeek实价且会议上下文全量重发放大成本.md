# bega 会议计费偏差：价格配置远低于 DeepSeek 实价 + 会议上下文全量重放的结构性放大

## 现象

用户 16:15 在 bega-labs 开了一场 bega 团队会议（6 个 bega-* agent），实际被 DeepSeek 扣费 3 元多，mindx 本地计费只算出 1.45 元，差 2.5 倍以上。

## 排查证据（token_usages.yml 实测聚合，2026-11-17 15:50~16:16）

- 182 次 LLM 调用，prompt 合计 19,364,533（缓存命中 18,653,952 占 96%，未命中仅 71 万），completion 181,635，reasoning 80,976。
- 本地计费公式（internal/core/cost.go CalculateCost）：未命中×in 价 + 命中×cache 价 + completion×out 价。按 ~/.mindx/settings/models.yml 配置（deepseek-flash: in ¥1 / cache ¥0.02 / out ¥2）算得 0.71+0.37+0.36 ≈ **¥1.45**，与用户「一块多」吻合。
- DeepSeek 官方实价（api-docs.deepseek.com/quick_start/pricing，deepseek-flash = V4.1-Flash-1017）：
  - peak：cache hit $0.006/M、miss $0.30/M、output $1.20/M；off-peak 减半。
  - peak 时段为 UTC 周一至周五 01:00–04:00 与 06:00–10:00；会议 16:15 CST = 周二 08:15 UTC，**落在 peak**。
- 按 peak 实价回算：miss 0.716×$0.30 + hit 18.654×$0.006 + out ~0.18~0.26×$1.20 ≈ **$0.55~0.64 ≈ ¥3.9~4.6**，与用户「3 块多」吻合。

## 根因

1. **models.yml 价格配置系统性偏低**（疑似按 off-peak 或旧价折算）：output 配 ¥2 vs peak 实价 ≈¥8.6/M（低 4 倍）；miss 配 ¥1 vs ≈¥2.1/M（低 2 倍）；cache 配 ¥0.02 vs ≈¥0.043/M（低 2 倍，恰≈off-peak $0.003）。缺口主要来自 output 与 miss。
2. **会议结构性成本放大**：每个参会 agent 每轮全量重发整场会议上下文（单次 prompt 12~14 万），6 agent × 高频轮次 → 15 分钟烧 1936 万 prompt tokens，呈二次增长（如 bega-integration 42 次调用累计 489 万 prompt）。
3. 本地计费未区分 peak/off-peak（DeepSeek 分时计价），任何单一价格配置都无法同时匹配两个时段。
4. 推理 token（reasoning 8.1 万）单独记录；若 completion_tokens 口径不含 reasoning，本地计费还漏掉这部分输出计费（影响约 ¥0.2，次要）。

## 识别信号 → 下次行动

- 下次看到「本地计费与供应商实扣对不上」，先做三步：聚合 token_usages.yml 分解 cached/uncached/completion → 用供应商官方实价回算 → 核对 models.yml 的 cost_per_1m_* 三项；禁止先怀疑计费公式。
- 下次见到「会议/多 agent 协作烧钱异常快」，先查单次调用的 prompt 规模与调用次数（19M = 182 次 × ~10.6 万均值），确认是否上下文全量重放。

## 会议实际干了什么（会话文件实证，bega-labs/.sessions/）

- bega-lead（新会话 01M7M8DQ…）：16:01 接到任务「先开会达成一致、出数据迁移与 API 通路的工作计划」→ 自行核实现状约 10 轮（确认新库 12 表全 0 行、旧系统是 Badger 而非 SQLite）→ SubAgent 并行派 5 岗「会前摸底」→ CollectResults 收割 5 份报告 → 逐岗转述 + 自纠一个数字口径（183 条相册两种解释对撞）→ 产出计划（10 条技术项自裁、5 条留用户裁决）→ AskUser 收尾等待裁决。
- 5 岗摸底各跑了 24~42 次 LLM 轮次，产出 5 份报告落盘 bega/.agents/reports/R-ALIGN-*.md（合计约 118KB），内容为真实验证：库表勘察、API 全组探测、go build + serve 实测、docker 核查、数据差异比对。
- 关键事实：5 个成员 agent 复用了 11 月 4—13 日就存在的老会话（各积 444~846 条消息、60~85 万字符），所以首次调用 prompt 就 128k+；bega-lead 是新会话，首次仅 58k。

## 成本结构真相（峰值实价回算 ≈$0.55~0.64）

- 1936 万 prompt 中 96% 是缓存命中，实价仅 $0.11——「19M 消耗」是纸面数字，不是账单主体。
- 真金白银 = 每轮新增内容（未命中 71.6 万 × $0.30 = $0.21）+ 输出（18~26 万 × $1.20 = $0.22~0.32）。即账单 ≈ 182 轮 × 每轮真实工作内容的全价。
- 结论：无失控循环；烧钱根因是「为一个计划触发五岗全面摸底」的编排选择 + 老会话巨型上下文（对费用影响小，对首字延迟与上下文质量有影响）+ 单条工具结果可达 2.8~3.8 万字符直接进历史。

## 用户拍板记录

- 价格修正与分时计价：均暂缓，不做供应商私有分时机制（避免独立依赖）；仅记录。
- 会议上下文优化：先出优化方案再定。
