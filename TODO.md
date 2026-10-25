# TODO

## 组织信协作机制再造——一期代码已落地，待运行期验证

- [x] daemon 六项改动全部完成（2026-10-25，go vet 全绿）：wakeForMail → mailboxKeeper（assistant 注册表验证 → 首员回退）+ 固定模板通知；enqueueOneJobNow 下一秒级 OneShot；地址上报停用；mail.deliver 移出 openMethods；send_message.go 删除（含 app.go 注册段、runtime import、toolmap.go 映射换 Notify）；ui_tools.go 新增 Notify 工具（user_notify 广播，message + level）
- [x] mindx-work：chatflow/store.ts 新增 user_notify 处理 → 非阻塞 ElMessage（high/medium/low/normal ↔ error/warning/success/info，grouping 防刷屏），vue-tsc 全绿
- [x] mail-in 技能进化 v1.1.0（validate 通过，履历已归档含 sha 签名）：凭编号取信、存量信补处理、组织位置 + USER.md 判读、截止时刻自查、TODO.md + Notify 兜底
- [ ] 运行期验证三件事：重启 daemon 后实测「注册表首员回退 + 雇佣校验」相容性（回退目标未雇佣时通知入队会失败并留 Warn 日志，信留存待技能补处理）；实测下一秒 OneShot 恰逢 watchLoop 5 秒重载间隙是否丢条目；TODO.md 落点路径当前为「项目 TODO.md」，组织信不绑项目目录时的具体路径待实测后定死
- [ ] 二期候选：直连秒达重接线（恢复地址上报 + mail.deliver 带发信方令牌经服务端内省核验后回 openMethods）
