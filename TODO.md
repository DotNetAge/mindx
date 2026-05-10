# TODO


- [x] 将Agent上的全部事件接入到Gort服务中；
- [x] 通过对话切换不同的Agent角色;
- [x] 系统启动时可以将Whisper
- [x] 是否可学OpenClaw增加一个Heatbeat，每几分钟就发一个心跳包到服务器，服务器收到心跳包就会去检查当前的Agent是否处于空闲状态，如果空闲的话就叫他去检查一下有没有事没有完成；
- [x] 要搞清楚当Tools的参数比较多的时候是其描述需要的增加参数说明吗？还是说编写一个原子化的技能来的使用这个Tools? 最主要是什么时候才适宜于用原子化的SKill定义？
- [ ] 形成客户/服务器的对话形式，每个客户端可以有一个身份标识；
- [ ] 实现人机合作式的项目管理，人通过客户端与主服务器中项目经理对话；
- [x] 将Project Tools 融合至各个技能之内，以“角色+技能”驱动的形式来开发这个项目管理；
- [x] 实现BoltSessionStore的存储，将会话存储在数据库中，而不是内存中；
- [ ] 实现基于GoRAG的Memory，实现语义化的技能查找；
  - [ ] 做GoRAG的实验，看看将ToolsRegistry索引进GoRAG，看看是否能正常召回；
  - [ ] 实现LongTermMemory的存储，提供对话经验的存储与检索；
- [x] 增加/修复创建Agent的Skill；
- [x] 将project系列的技能全部变成python脚本；
- [x] 更新Model与Agent的配置；
- [x] 发布GoGraph的python版本；
- [x] 增加设置Model配置的skill;
- [x] 增加一个基于Markdown.md的`RuleRegistry`;
- [x] 根据Agent的领域加载不同的SKILL;
- [x] 为了可以支持npx，runtime目录就应该固定在 ~/.mindx

---
- [x] 文件型的Session可能需要整调成为yml或xml, 当前的结构不利于重新加载；
- [x] TUI在 “⏱ 17m │ 💬 0 条消息” 这个栏位后面增加增加显示当前总共消耗的Tokens，如：“⏱ 17m │ 💬 0 条消息 | 20K Tokens”， 用K（千）为单位；
- [x] 如果能在  “● Connected                                                  @architect │ qwen3.6-plus” 这行消息栏的上方加一个条分隔线界面会更加清晰；
- [x] 当我输入“@”时就应该弹出"Agent Name Sugguestion"让我可以选，而且你要确认，“@<agent name> <content>” 这种格式输入是要自动切换agent成为当前对话的Agent的。
- [ ] Models, Skills 与 Agents 目录要增加“热重载”能力，当这些文件一但发生变化就自动重新加载其注册表；
- [ ] 增加一个修改“system-helper”的技能，这个技能用于帮助用户去设置环境变量，例如各个大模型所需要的API_KEY，其它需要从环境变量中读写值（用python实现）
