# 知识图谱（GraphRAG）

图谱以节点和边的形式存储**实体关系**。这里是结构化知识的存放地 —— 客户、项目、任务、营销活动以及它们之间的关联。LLM 的强项在于编写**动态 Cypher 查询**，这是人工难以手工组合的。

**所有命令都需要守护进程处于运行状态。**

## 核心操作

### 查询（读取）

| 任务 | 命令 | 说明 |
|------|------|------|
| 执行 Cypher 查询（只读） | `mindx graph query --cypher "MATCH (n) RETURN n LIMIT 10"` | 类似 SELECT，只读操作 |
| 传递参数 | `mindx graph query --cypher "MATCH (n {id:$id}) RETURN n" --params '{"id":"abc"}'` | 参数化查询 |
| 获取单个节点 | `mindx graph get-node --id <node-id>` | 按 ID 快速查找 |
| 查找邻居节点 | `mindx graph neighbors --id <node-id> --depth 2` | 遍历关系 |
| 限制邻居结果数 | `mindx graph neighbors ... --limit 20` | 限制返回数量 |
| 按边类型过滤 | `mindx graph neighbors ... --types MANAGES,HAS_GOAL` | 仅返回特定关系类型 |

### 写入（变更）

| 任务 | 命令 | 说明 |
|------|------|------|
| 执行 Cypher 写入 | `mindx graph exec --cypher "MATCH (n {id:'x'}) SET n.status='active'"` | CREATE/SET/DELETE/MERGE |
| 批量 Upsert 节点 | `mindx graph upsert-nodes --nodes '[{...},{...}]'` | 创建或更新多个节点 |
| 批量 Upsert 边 | `mindx graph upsert-edges --edges '[{...},{...}]'` | 创建或更新多条边 |

## 节点与边的数据结构

### 节点 JSON 格式
```json
{
  "id": "unique-id",
  "labels": ["EntityTypeName"],     // 例如 ["Customer", "Account"]
  "properties": {
    "name": "Display Name",
    "description": "...",           // 始终存在
    "confidence": 0.9,              // 始终存在 (0-1)
    // 自定义业务字段：
    "status": "active",
    "health_score": 72,
    "arr": 50000,
    "tier": "enterprise"
  }
}
```

### 边 JSON 格式
```json
{
  "from_node_id": "source-node-id",
  "to_node_id": "target-node-id",
  "type": "RELATIONSHIP_TYPE",      // 例如 MANAGES、HAS_GOAL、DEPENDS_ON
  "predicate": "human-readable description of this relationship",
  "properties": {
    "description": "...",
    "confidence": 0.9,
    // 自定义字段：
    "since": "2026-01-15",
    "weight": 1.0
  }
}
```

## 常见用法

### 构建项目结构
```bash
PROJ_ID=$(mindx utils uuid)
GOAL_ID=$(mindx utils uuid)

# 创建项目节点
mindx graph upsert-nodes --nodes "[{
  \"id\":\"$PROJ_ID\",
  \"labels\":[\"Project\"],
  \"properties\":{\"name\":\"App Launch\",\"status\":\"active\",\"progress\":0.0}
}]"

# 在项目下创建目标
mindx graph upsert-nodes --nodes "[{
  \"id\":\"$GOAL_ID\",
  \"labels\":[\"Goal\"],
  \"properties\":{\"title\":\"100k Users\",\"weight\":1.0,\"status\":\"pending\"}
}]"
mindx graph upsert-edges --edges "[{
  \"from_node_id\":\"$PROJ_ID\",\"to_node_id\":\"$GOAL_ID\",\"type\":\"HAS_GOAL\"
}]"

echo "Project: $PROJ_ID  Goal: $GOAL_ID"
```

### 追踪客户健康度（customer-success Skill 模式）
```bash
# 创建客户账户
CUSTOMER_ID=$(mindx utils uuid)
mindx graph upsert-nodes --nodes "[{
  \"id\":\"$CUSTOMER_ID\",
  \"labels\":[\"Customer\",\"Account\"],
  \"properties\":{
    \"company\":\"Acme Corp\",
    \"tier\":\"enterprise\",
    \"arr\":120000,
    \"health_score\":78,
    \"status\":\"active\"
  }
}]"

# 之后 —— 更新健康分数
mindx graph exec --cypher "
  MATCH (c:Customer {id:'$CUSTOMER_ID'})
  SET c.health_score = 72, c.updated_at = timestamp()
  RETURN c.company, c.health_score
"

# 查询有风险的客户
mindx graph query --cypher "
  MATCH (c:Customer)
  WHERE c.health_score < 60 AND c.status = 'active'
  RETURN c.company, c.tier, c.health_score, c.arr
  ORDER BY c.health_score ASC
  LIMIT 20
"
```

### 跨层查询：图谱 → 记忆
```bash
# 1. 在图谱中查找节点
mindx graph query --cypher "MATCH (p:Project {name:'App Launch'}) RETURN p.id"

# 2. 利用其来源文档搜索记忆
mindx memory query "App Launch requirements decisions" --min-score 0.7
```

## LLM 编写 Cypher 的要点

由于你（LLM）需要动态编写 Cypher 查询：

1. **用户输入务必使用参数化值**（`--params`），防止注入攻击
2. **读取用 `query`，写入用 `exec`** —— 两者权限级别不同
3. **批量操作使用 `upsert-nodes/upsert-edges`** —— 比逐条 Cypher SET/MERGE 更高效
4. **节点 ID 必须唯一且稳定** —— 新实体请使用 `mindx utils uuid` 生成
5. **标签就是你的索引** —— 先按标签查询，再过滤属性
6. **`neighbors` 专为图遍历优化** —— 深度优先探索时优先使用它，而非手动拼接 MATCH 链
