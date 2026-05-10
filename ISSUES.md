## TUI 

- [x] 工具执行显示栏位可以显示出当前在执行哪个工具，但应该没有进行换行导致出现信息完全堆在一起如： WebSearchWebSearch执行完成**这里是后面Thinking的内容.....
- [x] 如果后端执行失败后会导致再发送信息就 出现“⏱️ 请求超时
   建议: 检查网络连接或稍后重试” 的信息；只能重新载入TUI才能继续使用；
- [x] Suggestion 仍然只能显示出一行，可以自动筛选但只能打字才能看到有什么指令；应该同时显示五行的 Command Suggestion,一边输入一边筛选被匹配到的工具;
- [x] 由于Server改用了JSON-RPC进行通信，因此`schedule-cron` 和 `project-manager` 两个技能中与Server通信的调用就要重新适配当前的通信方式；
- [x] 运行时需要等服务器端补充事件中的 session_id 字段，否则 extractSession 返回空字符串，事件通过 LatestAnswer() 回退路由（单会话场景下正常工作）。
- [x] FileSessionStore 必须采用YML的方式进行存储，否则很难重建会话；