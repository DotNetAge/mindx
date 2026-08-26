# 调整对话流的系列组件


- [x] conversation.go 有非常严格的数据结构的错误，应该被取代并删除，也就是说convlist, conv_test 这些相关的类都一并被废弃；（已删除 conversation.go / convlist.go / conv_test.go / thinking.go / output.go / builder.go，由 stream.go 的事件驱动模型取代）
- [x] 正确对话流的显示逻辑是基于JSON-RPC事件驱动进行显示：（stream.go：Stream 单会话线性时间线 + StreamList 按 SessionID 路由）
  - [x] 思想流事件到达 用 Thinking 进行流式显示；完整显示整个思想流；用小斜体显示；（thinking_delta 追加至 ThinkingBlock，全文以灰色斜体呈现，结束后显示耗时）
  - [x] 动作流(ToolXXX事件) 用 Action 组件显示，该组件的显示逻辑是：
    - 开始执行直接显示 工具名(参数1，参数2，...) | 计时 | Token消耗 (服务器计算好的真正的Token消耗量，即输入+输出-缓存)（action.go：执行中实时计时，tool_exec_end 回填 Tokens）
    - 执行完成后显示输出结果（这里要预留格式化扩展，因为不同工具返回的结果的输出内容格式不同，需要输出显示的方式也不同）（RegisterResultFormatter(toolName, fn) 扩展点，未注册走默认按行渲染；支持折叠）
    - 如果工具执行错误，显示error组件；（失败结果渲染为红框错误组件）
  - [x] 结果 (Content)，当完成思考返回 Content 后用 output 组件显示；（content_delta / final_answer / task_summary → OutputBlock，markdown 渲染 + 分隔线，流式带光标）
  - [x] 当收到 AskUser 的事件时，先输出要Ask的问题，然后就是 Choices 组件显示用户可以选择的选项； 此时输入栏是不可见的，只有用户选定了结果并发送给服务端后，Choices消失，输入栏重新可见。（AskUserEventMsg → 问题写入对话流 + askChoices 内联面板；View 中底部区域互斥：授权栏 > Choices > 输入栏；ChoiceSelectedMsg 组装答案作为用户消息重入循环）
  - [x] 当收到 AskPermission 事件时，输入栏消失，当用户先定同意，拒绝 的选项后，向服务器发出对应的魔术词，并重新显示输入栏。（沿用 PermissionBar，输入栏隐藏与恢复统一在 View 的 bottomArea 互斥逻辑中）

## 对工具输出结果的格式化

以下工作都不需要完全输出其原JSON字符串

- [x] LS - 以列表形式格式化显示结果（解析 items[].name/type，目录加 "/" 后缀逐行列出，>50 条折叠为 "… +N"）
- [x] Grep - 以列表形式格式化显示结果（逐行列出，>30 行折叠）
- [x] Bash - 只需要显示执行结果 stdout 的内容  exit_code 为 1 时输出结果全变红。（实现为非零退出码整体标红，涵盖超时/被拦截等失败态；stdout 为空时回退展示 stderr；连续空行折叠，>20 行截断）
- [x] Write - 只显示 成功 / 失败 | + <共写入的行数> （行数取自调用参数 content，服务端 result 仅提供字节数）
- [x] Edit -  只显示 成功 / 失败 | + <共编辑的行数> - <共删除的行数> （± 行数分别取自 new_string / old_string 参数）
- [x] Read - 只显示 成功 / 失败 | <共读取的行数> （优先 lines_read 字段，缺失时按 content 行数兜底）
- [x] Skill - 只显示 成功 / 失败 | <共执行的行数> （指令文本 content 的行数）
- [x] WebSearch - 显示找到多少条结果 ： 共找到<结果数>条符合条件的结果 （统计 markdown 中 "### N." 标题条目数）
- [x] WebFetch - 只显示 成功 / 失败 | <共获取的行数> （大页面落盘形态直接读头部「行数：N」；小页面剥离头两行后统计正文行数）

实现位置：[formatters.go](formatters.go) —— 经 RegisterResultFormatter 注册，结构化工具的 JSON result 解析键名与服务端 goharness/tools 的 json tag 一一核对（bytes_written / replace_count / lines_read / total_items / exit_code 等）；执行失败的结果不进 formatter，仍由错误组件呈现。
文案：i18n 新增 conv.tool.success / conv.tool.failed / conv.tool.websearch.found（zh / en / zh-TW）。


## 发现的问题

请检查 Thinking 是否以流的形式显示思想流，而不是一次全部显示。
如果以流的形式显示，那么在获取到ThinkingDeltaMsg事件后，都需要更新 Thinking 组件的内容。

- [x] 已核查：是流式显示。链路为 rpc.go（每条 thinking_delta → ThinkingDeltaMsg）→ client.go Update → streamList.Update 按 SessionID 路由 → UpdateStream 对活跃 ThinkingBlock 执行 `Text += delta` 增量追加，随后 bubbletea 重渲染对话视口。每个事件都更新组件内容，无需修改。

- [x] 输入框内的键入 "/model" 指令后，显示的模型列表信息完全错位了，是否应该根据EML体系进行重新设计，将其做成 Alt ? 又或者是按其原有的设计方式将其位置进行重新调整显示于输入框下方？ 你以此如何看，因为这个模式模式同时影响了 "/agent" 的切换指令，它们都应该是同构的；