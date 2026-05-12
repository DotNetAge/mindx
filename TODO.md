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
- [ ] 增加一个修改“system-installer”的技能，这个技能用于帮助用户去设置环境变量，例如各个大模型所需要的API_KEY，其它需要从环境变量中读写值（用python实现）
- [ ] 完整记录大模型推理链路，建立基于UI的会话，UI会话与ReAct的会话的差异在于,
  - [ ] UI会话将会记录用户的问题与大模型的完整推理链路，作为日后从该链路中重新总结并推理出“经验”而获得新Skill的基础（目标是减少推理次数，提高推理的质量）。
  - [ ] UI会话也是用于重建TUI界面让用户可以回顾会话历史的过程；
- [ ] Demon 中的对话指令应该是：`@agent_name <session_id> <content>`
- [ ] 创建一个 Evolve 技能，作用: 
  - 列出所有技术，反思有哪些技能可以装配到自己技能列表`skills`中，以完全匹配当前的岗位职责。
  - Beta: 从推理链条中反思，有哪些会话是用户经常性需要进行处理的事，可以将这些事件的推理过程进行仿照甚至优化，以达到更好的推理效果，从而提炼成为新的技能；
- [ ] 仔细阅读Claude的代码，了解一下Claude是怎么样让大模型自己去判断权限的，然后如何通过权限判断的思考结果来向用户发起授权请求；
- [x] 自装配，当 mindx 运行时会首先自检:
  - [x] 工作目录(用户目录)是否存在，如果不存在就要构建原始的工作目录；
    - [x] 问题：go:embed 的文件是否可以copy到真实的物理目录中？
  - [x] (<工作目录>/mindx.json)，检查是否已经进行过初始化
    - 最后一次对话的 agent 名 <- 根据此名称设置当前的Agent
    - 最后一次对话的 session_id <- 根据此ID设置加载对应的Session
    - 是否已配置 dameon 服务
    - 是否有默认 model (配置默认model会重置全部Agent的model配置) 如果没有配置Model与API_KEY 进入配置界面，第一步选择默认模型（从models.json中加载）, 第二步选择输入API_KEY, 将用户的API_KEY直接填写到models.json中，并且将全部的Agent的model配置都设置为默认模型名称，然后保存全部的Agent配置;
    - 是否已配置 API_KEY
    - 进入正式的加载过程，加载Agent, 加载Sessions 等等；
  - [x] FileSessionStore 有一个问题，就是没有为Session建立独立的目录，而且每个SessionID都是有问题的，不是按照时间顺序的, 需要将Session目录根据时间戳来创建，另外session文件名就是`<session_id>.yml`

---

- [x] Demon 需要Hold住与Client端完全一至的Workspace, 否则对于计划任务的执行就会产生目录的偏移，可能会导致文件找不到或目录不正确的错误；因此，客户端是通过`os.Getwd()`来获取当前工作目录，而需要有一个手段来设置Demon的工作目录，以确保Demon与Client端是完全保持一至。
  - 思路1: 将 SessionID 与 工作目录绑定，一个会话就必须与一个工作目录绑定；