# 发布工作流 Homebrew 公式 sha256 为空：needs 抢跑 + 空值静默

## 现象

重推 v2.6.21 时发现 tap 仓库 `Formula/mindx.rb` 里两个 `sha256 ""` 全为空，`brew install` 必然校验失败。回溯历史版本（v2.6.20、v2.6.19、v2.6.17）同样为空——不是本次重发引入，是自 v2.6.17 起一直坏的，只是没人验证过公式内容。

## 根因（两条叠加）

1. `homebrew` job 的 `needs: [prepare]`。tag 推送模式下它取的是 build job 的产物 `package-darwin-*`，但 build 还没跑完它就开工，`actions/download-artifact` 空手而归，`dist/` 是空的。
2. `Calculate SHA256` 步骤没有 fail-fast：

```bash
SHA256_AMD64=$(shasum -a 256 "dist/..." | cut -d' ' -f1)
```

文件不存在时 `shasum` 只往 stderr 报错，管道退出码取的是 `cut` 的 0，赋值成功但值为空串。于是 sed 把 `__SHA256_AMD64__` 换成空串，公式变成 `sha256 ""`，job 一路绿到底，还把坏公式推到 tap 仓与 Release 资产。

同源缺陷：Homebrew 的 fallback 步（`softprops/action-gh-release`）在 Release 尚未创建时会自建残缺 Release（v2.6.11 首跑实证，产物只有 mindx.rb、正文为空），也是 `needs` 缺 `release` 导致的时序问题。

## 修复

- `homebrew` job 改为 `needs: [prepare, build, release]`：tag 模式等产物、dispatch 模式等 Release，两种触发都不会抢跑。
- 校验和步骤加 `set -euo pipefail`，并对两个变量显式 `test -n` 断言，空值当场失败。

## 已做的补救（v2.6.21 当次）

旧流水线已经推出坏公式，代码修复只对后续版本生效，当次数据需手工订正：

- 从 Release 的 `checksums.txt` 取两个 darwin 产物的真实 sha256（不要去下载 17MB 包再算，checksums 就是发布时对同一批资产算出来的）。
- 本地用模板 + sed 生成正确公式，`gh release upload v2.6.21 mindx.rb --clobber` 覆盖 Release 资产。
- 用 `gh api -X PUT repos/DotNetAge/homebrew-mindx/contents/Formula/mindx.rb` 带上 `.sha` 更新 tap 仓库，并在 commit message 里标注「修正 sha256」。

## 下次识别信号

下次看到「发布 job 失败/产物异常」或新增会读取构建产物、Release 资产的 job 时，先核对它的 `needs` 是否覆盖了真正产出该资产的上游 job，再检查所有「命令替换 + 管道」的赋值步骤有没有 `set -euo pipefail` 与非空断言——空值静默通过是这类流水线缺陷的通用藏身处。
