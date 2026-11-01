# TODO

## project_dir 空值兜底清除（2026-11-01）

- [ ] 重启 daemon 使修复生效：当前运行中的 ~/.mindx/bin/mindx（pid 见 lsof，构建于 2026-11-01 12:13）仍带 Getwd 兜底；`go build -o ~/.mindx/bin/mindx ./cmd` 后 `mindx restart`。重启会打断运行中会话，需用户确认时机
- [ ] 全新 macOS 复测首装链路：选工作区 → 首条消息懒建会话，验证会话落在所选目录、~/.mindx 下无 .sessions
- [ ] mindx-work 对话流不显示 Agent 头像与 NickName：代码链路（fetchInitialData → agents.registry → agentIdentityMap → ChatRoundView agent-row）形式完整、daemon agent.list 有 nick_name/icon 字段，根因未定位；需用户提供具体表现（整行缺失 vs 行内无头像）或 CDP 实测

## 令牌落点归一收尾（2026-10-30 定案）

- [ ] 发布 mindx 新 release（本地多个未推提交：令牌归一 + 注释同步 + Origin 白名单删除 + 安装链路审计修复 + Windows 构建目标，以 `git log origin/main..main` 实数为准）——**必须先于 mindx-work 新壳发版，顺序不能反**：push main + tag 触发 release.yml（5 平台构建 + docker 镜像自动推 Docker Hub）；验收：release 资产含 mindx-<v>-windows-amd64.zip 且 checksums.txt 覆盖全部资产、打包期 build-mindx-bundle 拉到新 latest、干净机首装 `.token` 单点生效、向导模型步不再弹「智能主机连接失败」红条。**注意：新 daemon 将拒绝 file:// Origin，必须与 mindx-work 连接归主进程的新壳成对发版，旧壳用户升级 daemon 即断连**

## 组织信协作机制再造——一期代码已落地，待运行期验证

- [ ] 运行期验证三件事：重启 daemon 后实测「注册表首员回退 + 雇佣校验」相容性（回退目标未雇佣时通知入队会失败并留 Warn 日志，信留存待技能补处理）；实测下一秒 OneShot 恰逢 watchLoop 5 秒重载间隙是否丢条目；TODO.md 落点路径当前为「项目 TODO.md」，组织信不绑项目目录时的具体路径待实测后定死
- [ ] 二期候选：直连秒达重接线（恢复地址上报 + mail.deliver 带发信方令牌经服务端内省核验后回 openMethods）

## TUI 对话流

- [ ] 用户验收：思考流无标签直出（renderThinkingNode 重写后）、stderr 日志穿透消除（WithConsoleLog(false)）——若「行与行粘死的残影」仍存在，下一步专攻 bubbletea renderer 行高记账（emoji/CJK 宽度测量与实际渲染行数不符、viewport 高度估计与实际 header/footer 行数偏差）
