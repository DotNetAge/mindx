---
version: "2.6.4"
pub_date: 2026-11-24
sha: "b00f0a11b264b87bd96fec30a9a2cc527288283ba71dc2e3b8fbedf9e48b6125"
modifier: "Ray"
modifier_id: "3a05e6a6-b478-4595-8dd1-6fa3b54f75bc"
---

## 2026-11-24 脚本找不到 mindx CLI（retrospect）

- 问题：流程漏洞——`scan.py` 的 `run_json` 把命令写死成 `["mindx", ...]`，依赖调用方 PATH 里有 `mindx`。Agent 工具环境不保证继承用户 shell 的 PATH：当前 MainAgent 环境 `command -v mindx` 返回 127，二进制实际在 `~/.mindx/bin/mindx`，于是 `scan.py debt` 报 `会话清单不可用：[Errno 2] No such file or directory: 'mindx'`、`checked: 0`、`over: []`。这会把「扫描失败」伪装成「没有债务」，比直接报错更危险。
- 否决：要求用户先把 `~/.mindx/bin` 加进 PATH 再跑技能。否决理由：技能要能在任意 Agent 环境自包含执行，不能依赖宿主 shell 的预配置；同一次自省在 go-programmer 环境能跑、在 MainAgent 环境不能跑，说明是解析方式不稳定。
- 修正：新增 `mindx_exe()`，解析顺序为 `MINDX_BIN` 环境变量 → `shutil.which("mindx")` → `~/.mindx/bin/mindx` → 裸 `mindx`；`debt` 的两个 RPC 调用与 `compact_command` 统一使用解析后的 `MINDX`。version 2.6.3 → 2.6.4。
- 验证：当前 MainAgent 环境 PATH 无 `mindx`，修复后 `scan.py debt` 返回 `checked: 2`、不再报 `No such file or directory`；显式设置 `MINDX_BIN` 指向同一二进制同样能跑通。
