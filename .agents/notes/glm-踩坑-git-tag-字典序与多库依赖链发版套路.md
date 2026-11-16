# 踩坑：git tag 字典序陷阱与多库依赖链发版套路

## 问题
1. `git tag -l | tail -5` 按字典序输出，`v2.6.10` 排在 `v2.6.5` 之前，导致误判最新 tag 为 v2.6.9，连打两次 "tag already exists"（先试 v2.6.10 又试 v2.6.11 均已存在，真实最新是 v2.6.26）。
2. mindx 仓库同时存在名为 `main` 的分支和 tag（本地与远端皆然），`git push origin main` 报 "dst refspec main matches more than one"，且 `git log origin/main..main` 触发 "refname 'main' is ambiguous" 警告，输出可能为空。

## 结论
- 下次我看到要查 git 仓库最新版本 tag，我将用 `git tag -l --sort=version:refname | tail`（或 `--sort=-creatordate`），绝不裸用 `| tail`。
- 下次我看到 mindx（或任何存在同名分支+tag 的仓库）需要推送分支，我将用完整 refspec：`git push origin refs/heads/main:refs/heads/main`；引用 commit 用 `refs/heads/main`。

## 依赖链发版套路（gort/gochat/goagent/goharness/mindx，2026-11 实测可行）
1. 自底向上：gort、gochat（无内部依赖）→ goagent（依赖 gochat）→ goharness（依赖 goagent+gochat）→ mindx。每库：go get 升下游依赖 → `go build ./... && go test ./...` 绿 → 提交 → push → 打 patch 新 tag → push tag。
2. mindx 收尾：先把无关改动（单测批次）独立提交推送，再 `go get` 四个库到新 tag + `go mod tidy`，全量测试绿后单独提交，最后打 tag。
3. 各库 HEAD 若已有 tag 且仅有 toolchain 行改动，仍按用户拍板 patch 递增重打 tag，保持链路自洽。

（updated: 2026-11-15）
