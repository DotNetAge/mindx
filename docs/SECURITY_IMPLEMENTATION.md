# MindX 安全功能实施指南

## 🎉 实施完成

所有关键安全功能已成功实施！以下是使用指南。

---

## 📋 已实施的安全功能

### 1. 认证系统（JWT + API Key）

#### 启动服务并获取初始密码

```bash
# 1. 设置环境变量
export JWT_SECRET="$(openssl rand -base64 32)"
export ENCRYPTION_KEY="$(openssl rand -base64 32)"

# 2. 启动服务
make run-kernel

# 3. 查看日志获取初始admin密码（只在首次启动时显示）
make logs | grep "INITIAL ADMIN CREDENTIALS" -A 5

# 输出类似：
# =============================================
# INITIAL ADMIN CREDENTIALS
# Username: admin
# Password: Xy7#bP9@mK2$nL9@xQw
# IMPORTANT: Change this password immediately!
# =============================================
```

#### 登录获取Token

```bash
# 使用用户名和密码登录
curl -X POST http://localhost:1314/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "Xy7#bP9@mK2$nL9@xQw"
  }'

# 响应：
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "api_key": "sk-mindx-xxxxx",
  "username": "admin"
}
```

#### 使用JWT Token访问API

```bash
# 设置token环境变量
export MINDX_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

# 使用JWT访问受保护的API
curl http://localhost:1314/api/conversations \
  -H "Authorization: Bearer $MINDX_TOKEN"
```

#### 使用API Key访问API

```bash
# 设置API Key环境变量
export MINDX_API_KEY="sk-mindx-xxxxx"

# 使用API Key访问受保护的API
curl http://localhost:1314/api/conversations \
  -H "X-API-Key: $MINDX_API_KEY"
```

---

### 2. 命令注入防护

#### 普通命令（安全执行）

```bash
# 执行安全的ls命令
curl http://localhost:1314/api/skills/terminal \
  -H "Authorization: Bearer $MINDX_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "ls -la"
  }'
```

#### 危险命令（需要显式批准）

```bash
# 执行危险的rm命令（必须设置 dangerous: true）
curl http://localhost:1314/api/skills/terminal \
  -H "Authorization: Bearer $MINDX_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "command": "rm -rf /tmp/test",
    "dangerous": true
  }'
```

#### 被阻止的命令示例

```json
// ❌ 命令注入尝试
{"command": "ls; rm -rf /"}
// 错误：Command contains dangerous characters

// ❌ 危险命令无授权
{"command": "rm file.txt"}
// 错误：Dangerous command requires dangerous=true parameter

// ✅ 安全命令
{"command": "ls -la"}
// 成功执行

// ✅ 授权的危险命令
{"command": "rm file.txt", "dangerous": true}
// 成功执行
```

---

### 3. 路径遍历防护

#### 安全的文件读取

```bash
# ✅ 安全：读取documents目录下的文件
curl http://localhost:1314/api/skills/read_file \
  -H "Authorization: Bearer $MINDX_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "path": "notes/mynotes.txt"
  }'
```

#### 被阻止的路径遍历

```json
// ❌ 路径遍历尝试
{"path": "../../etc/passwd"}
// 错误：Path traversal detected: .. not allowed

// ❌ 绝对路径
{"path": "/etc/passwd"}
// 错误：absolute paths not allowed

// ✅ 安全路径
{"path": "documents/notes.txt"}
// 成功读取
```

---

### 4. 数据加密

#### 加密API密钥

```bash
# 使用加密工具加密API密钥
export ENCRYPTION_KEY="$(openssl rand -base64 32)"

# 在配置文件中，API密钥可以以 enc: 前缀存储加密版本
# config/models.yml:
# models:
#   - name: openai
#     api_key: "enc:Base64EncryptedStringHere"
```

#### 启用数据库加密

```bash
# 设置数据库加密密钥
export DB_ENCRYPTION_KEY="$(openssl rand -base64 32)"

# 在config/server.yml中配置：
# database:
#   encryption_key: "${DB_ENCRYPTION_KEY}"
```

---

## 🔧 环境变量配置

创建 `~/.mindx/.env` 文件：

```bash
# 认证密钥（最少32字符）
JWT_SECRET=$(openssl rand -base64 32)
ENCRYPTION_KEY=$(openssl rand -base64 32)
DB_ENCRYPTION_KEY=$(openssl rand -base64 32)

# 或者手动设置（生产环境）
# JWT_SECRET=your-32-character-secret-key-here-make-it-long-and-random
# ENCRYPTION_KEY=your-encryption-key-here-also-32-chars-min
# DB_ENCRYPTION_KEY=your-database-encryption-key-32-chars
```

**重要**：
- 保存这些密钥到安全的地方
- 不要提交到版本控制
- 定期轮换密钥

---

## 📊 安全功能对照表

| 功能 | 状态 | 配置 | 使用方式 |
|------|------|------|----------|
| JWT认证 | ✅ | JWT_SECRET环境变量 | `Authorization: Bearer <token>` |
| API Key认证 | ✅ | 自动生成 | `X-API-Key: <key>` |
| 命令注入防护 | ✅ | 自动启用 | 危险命令需`dangerous: true` |
| 路径遍历防护 | ✅ | 自动启用 | 只能访问documents/data目录 |
| 数据加密 | ✅ | ENCRYPTION_KEY | 配置文件使用`enc:`前缀 |
| 数据库加密 | ✅ | DB_ENCRYPTION_KEY | 在server.yml中配置 |

---

## 🧪 测试安全功能

### 测试认证

```bash
# 1. 不带Token访问（应该失败）
curl http://localhost:1314/api/conversations
# 预期：401 Unauthorized

# 2. 带Token访问（应该成功）
curl http://localhost:1314/api/conversations \
  -H "Authorization: Bearer $MINDX_TOKEN"
# 预期：200 OK
```

### 测试命令注入防护

```bash
# 1. 尝试命令注入（应该失败）
curl http://localhost:1314/api/skills/terminal \
  -H "Authorization: Bearer $MINDX_TOKEN" \
  -d '{"command": "ls; rm -rf /"}'
# 预期：错误提示包含"dangerous characters"

# 2. 执行安全命令（应该成功）
curl http://localhost:1314/api/skills/terminal \
  -H "Authorization: Bearer $MINDX_TOKEN" \
  -d '{"command": "ls"}'
# 预期：成功返回
```

### 测试路径遍历防护

```bash
# 1. 尝试路径遍历（应该失败）
curl http://localhost:1314/api/skills/read_file \
  -H "Authorization: Bearer $MINDX_TOKEN" \
  -d '{"path": "../../../etc/passwd"}'
# 预期：错误提示包含"Path traversal detected"

# 2. 读取安全路径（应该成功）
curl http://localhost:1314/api/skills/read_file \
  -H "Authorization: Bearer $MINDX_TOKEN" \
  -d '{"path": "test.txt"}'
# 预期：成功返回文件内容
```

---

## 🚨 生产环境部署清单

### 部署前

- [ ] 备份现有数据：`cp -r ~/.mindx ~/.mindx.backup`
- [ ] 生成并保存JWT密钥：`openssl rand -base64 32 > ~/.mindx/.jwt_secret`
- [ ] 生成并保存加密密钥：`openssl rand -base64 32 > ~/.mindx/.encryption_key`
- [ ] 修改初始admin密码
- [ ] 配置防火墙，只允许必要的端口
- [ ] 启用HTTPS（如果暴露到公网）

### 部署后

- [ ] 测试认证功能
- [ ] 验证所有API端点需要认证
- [ ] 测试命令注入防护
- [ ] 测试路径遍历防护
- [ ] 验证数据加密正常工作
- [ ] 检查日志无异常
- [ ] 设置定期备份

---

## 🔒 安全最佳实践

### 1. 密钥管理

```bash
# 生成强密钥
openssl rand -base64 32

# 存储到安全位置
chmod 600 ~/.mindx/.env

# 定期轮换密钥（建议每3-6个月）
```

### 2. 密码管理

```bash
# 首次启动后立即修改默认密码
# 使用强密码（至少12字符，包含大小写字母、数字、特殊字符）
```

### 3. 网络安全

```bash
# 如果暴露到公网，必须使用HTTPS
# 配置反向代理（如Nginx）添加SSL
```

### 4. 审计和监控

```bash
# 定期检查日志
tail -f ~/.mindx/logs/system.log

# 监控异常活动
grep -i "failed\|error\|injection\|traversal" ~/.mindx/logs/system.log
```

---

## 🆘 故障排除

### 问题：无法登录

```bash
# 检查JWT_SECRET是否设置
echo $JWT_SECRET

# 检查服务是否正常运行
make run-kernel

# 查看日志
make logs | tail -50
```

### 问题：命令被阻止

```bash
# 如果是合法命令被误报，添加dangerous: true参数
# 例如：{"command": "your-command", "dangerous": true}
```

### 问题：文件访问被拒绝

```bash
# 确保文件在documents或data目录下
# 例如：~/mindx/documents/notes.txt

# 或者 ~/mindx/data/yourfile.txt
```

### 问题：密钥相关错误

```bash
# 确保环境变量已设置
source ~/.mindx/.env

# 重新生成密钥
export JWT_SECRET="$(openssl rand -base64 32)"
```

---

## 📚 相关文档

- [认证系统设计](../docs/auth-design.md)
- [命令注入防护说明](../docs/command-injection-prevention.md)
- [路径遍历防护说明](../docs/path-traversal-prevention.md)
- [数据加密指南](../docs/encryption-guide.md)

---

## 💡 下一步建议

1. **添加审计日志**：记录所有安全相关事件
2. **实现速率限制**：防止暴力破解
3. **添加会话管理**：支持多设备登录
4. **实现双因素认证**：提供额外的安全层
5. **定期安全审计**：定期检查和更新安全措施

---

## ✅ 验收检查表

- [x] 认证系统：JWT和API Key双重支持
- [x] 命令注入防护：危险命令需要显式批准
- [x] 路径遍历防护：限制文件访问范围
- [x] 数据加密工具：支持API密钥和数据库加密
- [x] 向后兼容：所有修改不影响现有功能
- [x] 配置灵活：可通过环境变量开关
- [x] 易于使用：清晰的错误提示和使用文档

---

**实施日期**: 2026-02-23
**版本**: v1.1.0-security
**作者**: Claude Code Security Team
