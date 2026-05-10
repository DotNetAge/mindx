# Agentic Project Management Design


## 技能需求


### 聘用员工（hire-staff）

> 检查 {workspace}/agents/目录下是否有符合要求的员工，如果没有，则创建一个符合要求的员工；
要通过sh脚本来执行，因为当前skill的操作位置已经超出skill的目录范围，只能从全局环境变量 env=MINDX_WORKSPACE 中获取workspace的路径。然后从 {workspace}/agents/ 目录下，解释 agents的文档定义，逐一调取员工资料，然后生成一个符合要求的员工。

具体工作内容：

1. 描述【岗位名】以及该职位【职能描述】
2. 描述岗位的职责

### 工作分解（work-breakdown）

基于 wbs 工作分解理论，生成工作项