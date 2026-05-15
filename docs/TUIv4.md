
● 深度思考中 <- 处于等待时显示，显示时这个 ● 图标在白色与蓝色之间闪烁。 
---思考区--- 
● 用户让我总结的之前的搜索结果，我要.... <- Thinking 的内容（思想流）当前是错的显示的貌似是推理内容而不是思想流,要完整不要截断。
● 第二轮思考时的思想流 
● 第三轮思考时的思想流
● 如此类推
---工具调用---
⏺ Bash(go test ./internal/client/ -v 2>&1) | 预计消耗 10K Tokens <- 工具调用时 ● 图标在白色与绿色之间闪烁，完成后变成绿色。 
  ⎿ === RUN   TestMouseClickRestoresFocus <- 工具调用结果（最多显示三行，默认折叠，调用结果灰色小字）
     === RUN   TestMouseClickRestoresFocus/Click_restores_focus_when_unfocused
     === RUN   TestMouseClickRestoresFocus/Click_maintains_focus_when_already_focused
     … +30 lines (ctrl+o to expand)
  ⎿  (timeout 2m)
⏺ Bash(sed -n '771,785p' internal/client/component_root.go)  | 预计消耗 10K Tokens <- 第二个工具
  ⎿  func (m *rootModel) routeToAnswer(answer *AgentAnswer, contentType, content string) {
        switch contentType {
        case "thinking", "ThinkingDelta", "thinking_delta", "THINKING_DELTA":
     … +12 lines (ctrl+o to expand)
⏺ Bash(python3 << 'PYEOF' | 预计消耗 10K Tokens <- 第三个工具，如果工具执行出错 ● 图标显示为红色
      with open('internal/client/component_root.go', 'r') as f:…)
  ⎿  Error: Exit code 1
     Traceback (most recent call last):
       File "<stdin>", line 12, in <module>
     AssertionError: Expected 1 match, found 0
---最终回答区（即使有N轮思考循环，也只返回一个结果）---
⏺ 回答的第一行内容
这里是正文

### 说明

### ● 图标 的颜色说明
- 思想流的 ● 图标一律采用蓝色，深度思考中 ● 图标在白色与蓝色之间闪烁，完成后变成蓝色。
- 工具流的 ● 图标一律采用绿色，工具调用时 ● 图标在白色与绿色之间闪烁，完成后变成绿色，工具执行出错 ● 图标显示为红色。
- 最终回答 ● 图标一律采用白色；