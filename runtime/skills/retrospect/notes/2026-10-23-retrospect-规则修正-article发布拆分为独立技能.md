# 2026-10-23 retrospect 规则修正-article发布拆分为独立技能

## 定案

用户定案：article 发布是独立技能（article-publish，项目技能），retrospect 只判定候选并路由。v1.1.0 曾把发布流程（落位/登录三分支/推送）内嵌进步骤 3 第 5 条，属职责揉杂，回改为纯路由一行。

## 职责边界

- retrospect：判定知识形态（技能/文章/插件三分流）+ 文章质量门（论点/出处/说人话）→ 路由
- article-publish：落位、格式校验、登录检查、用户批准、推送、线上验证

## 关键事实

新技能落位后注册用 `mindx skill add <目录>`（自动重载）；`mindx skill reload` 子命令不存在（skill-creator 正文已同步修正）。
