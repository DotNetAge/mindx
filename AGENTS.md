# mindx 仓库规则

## 必须遵守

* 思考流与推理过程全部使用中文
* 结论先行，简短回答，像人类一样说话
* 禁止将假设或推测当作事实呈现；不确定时直接说明，不完整但诚实的答案优于完整但错误的答案
* 禁止使用表情符号

## daemon 更新与 E2E 验证（2026-10-04 用户定稿）

* 本地改代码后要让 daemon 用上新二进制，**必须使用 `make restart`**（构建 → 写入 `~/.mindx/bin` → launchd 强制重启 → 运行时同步，一条命令完成）；禁止手动 kill/重启 daemon 或手动拷贝二进制——编译必须在 kill 之前，`make restart` 已内置该顺序（KeepAlive=true 会在 kill 后立刻复活，复活的就是新二进制）

## runtime 目录铁律（2026-10-04 用户定稿，违反即事故）

* `runtime/` 是 `~/.mindx` 的初装种子：初装时由 `ExtractWorkspace`（internal/core/workspace.go）将嵌入 FS 中 `runtime/` 下的一切整体释放到用户主目录，运行期由 `SyncRuntimeAssets` 按版本同步——**runtime/ 下任何文件都是会到达用户机器的活配置，不存在"死配置"**
* 在 `runtime/` 下新增或移动文件时，**必须同步更新根目录 `embed.go` 的 `//go:embed` 清单**——清单未收录的文件进不了嵌入 FS，初装释放不到用户目录（2026-10-04 实锤：policy.json 加入 runtime/ 后漏更新清单，被误判为死配置）
* 用户可编辑的根级配置文件（如 `policy.json`）必须加入 `SyncRuntimeAssets` 的 `checkFiles` 保护列表（checksum 未变才随版本更新），防止版本升级覆盖用户改动
* 判断 runtime/ 文件的引用关系时，检索范围必须包含 `embed.go` 清单、`ExtractWorkspace`/`SyncRuntimeAssets` 的 WalkDir 逻辑与 `internal/setup` 的同步调用点，缺一处即结论无效

## 必须禁止

* 禁止未经核实断言 runtime/ 下文件的"引用/生效"状态——embedding、初装释放、版本同步三层机制都要查证
