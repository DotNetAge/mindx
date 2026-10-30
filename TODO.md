# TODO

## 令牌落点归一收尾（2026-10-30 定案）

- [ ] 发布 mindx 新 release（本地 2 个未推送提交：令牌归一 + 注释同步）——**必须先于 mindx-work 新壳发版，顺序不能反**：push main + tag v2.6.6 触发 release.yml（4 平台构建 + docker 镜像自动推 Docker Hub）；验收：打包期 build-mindx-bundle 拉到新 latest、干净机首装 `.token` 单点生效、向导模型步不再弹「智能主机连接失败」红条

## 组织信协作机制再造——一期代码已落地，待运行期验证

- [ ] 运行期验证三件事：重启 daemon 后实测「注册表首员回退 + 雇佣校验」相容性（回退目标未雇佣时通知入队会失败并留 Warn 日志，信留存待技能补处理）；实测下一秒 OneShot 恰逢 watchLoop 5 秒重载间隙是否丢条目；TODO.md 落点路径当前为「项目 TODO.md」，组织信不绑项目目录时的具体路径待实测后定死
- [ ] 二期候选：直连秒达重接线（恢复地址上报 + mail.deliver 带发信方令牌经服务端内省核验后回 openMethods）

## TUI 对话流

- [ ] 用户验收：思考流无标签直出（renderThinkingNode 重写后）、stderr 日志穿透消除（WithConsoleLog(false)）——若「行与行粘死的残影」仍存在，下一步专攻 bubbletea renderer 行高记账（emoji/CJK 宽度测量与实际渲染行数不符、viewport 高度估计与实际 header/footer 行数偏差）
