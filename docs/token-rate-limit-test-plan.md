# 令牌级别速率限制 - 测试计划

## 测试环境准备

### 1. 启动后端
```bash
# 确保 Redis 已启动
redis-server

# 启动后端
go run main.go
```

### 2. 启动前端
```bash
cd web
bun install
bun run dev
```

### 3. 数据库迁移
首次启动时，GORM 会自动迁移新增的字段。检查日志确认迁移成功。

---

## 测试用例

### ✅ AC1: Token 模型支持新字段

**测试步骤：**
1. 登录系统
2. 进入令牌管理页面
3. 创建新令牌
4. 查看令牌详情

**预期结果：**
- 可以正确读写 4 个新字段
- 数据库中正确保存

**验证方式：**
```bash
# 检查数据库表结构
sqlite3 new-api.db ".schema tokens"
# 或 MySQL
mysql -u root -p -e "DESCRIBE tokens;"
```

---

### ✅ AC2: 令牌级别限流生效

**测试步骤：**
1. 创建令牌，启用独立限流
2. 配置：10 次/1分钟
3. 使用该令牌在 1 分钟内请求 11 次

**测试脚本：**
```bash
# 设置令牌
TOKEN="sk-your-token-here"

# 发送 11 次请求
for i in {1..11}; do
  echo "Request $i"
  curl -X POST http://localhost:3000/v1/chat/completions \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
      "model": "gpt-3.5-turbo",
      "messages": [{"role": "user", "content": "Hello"}]
    }'
  echo ""
done
```

**预期结果：**
- 前 10 次请求成功
- 第 11 次请求返回 429 Too Many Requests
- 错误消息：`您已达到总请求数限制：1分钟内最多请求10次`

---

### ✅ AC3: 令牌独立计数

**测试步骤：**
1. 创建令牌 A，启用独立限流（10次/分钟）
2. 创建令牌 B，不启用独立限流
3. 令牌 A 请求 10 次
4. 令牌 B 请求 10 次
5. 令牌 A 再请求 1 次

**预期结果：**
- 令牌 A 的第 11 次请求被拒绝
- 令牌 B 不受影响

---

### ✅ AC4: 向后兼容

**测试步骤：**
1. 使用现有令牌（未启用独立限流）
2. 发送请求

**预期结果：**
- 继续使用分组/全局配置
- 行为与改造前一致

---

### ✅ AC5: 区分总请求数和成功请求数

**测试步骤：**
1. 创建令牌，配置：
   - 总请求数：20 次/分钟
   - 成功请求数：10 次/分钟
2. 发送 10 次成功请求
3. 发送第 11 次成功请求

**预期结果：**
- 第 11 次成功请求被拒绝
- 但失败请求仍可继续（直到总数达到 20）

---

### ✅ AC6: 前端表单正确显示和提交

**测试步骤：**
1. 打开令牌编辑页面
2. 启用令牌级别限流
3. 配置参数：
   - 总请求数：100
   - 成功请求数：80
   - 限流周期：5 分钟
4. 点击快捷设置按钮
5. 提交表单

**预期结果：**
- 表单正确显示配置字段
- 快捷设置按钮正确填充值
- 提交后数据正确保存

---

### ✅ AC7: API 正确返回新字段

**测试步骤：**
```bash
# 获取令牌详情
curl -X GET http://localhost:3000/api/token/1 \
  -H "Authorization: Bearer your-access-token"
```

**预期结果：**
```json
{
  "success": true,
  "data": {
    "id": 1,
    "name": "test-token",
    "rate_limit_enabled": true,
    "rate_limit_total_count": 100,
    "rate_limit_success_count": 80,
    "rate_limit_duration": 5
  }
}
```

---

### ✅ AC8: Redis Key 正确生成

**测试步骤：**
1. 创建令牌 ID 为 5001，启用独立限流
2. 使用该令牌请求
3. 检查 Redis

**验证方式：**
```bash
# 连接 Redis
redis-cli

# 查看所有限流 Key
KEYS rateLimit:token:*

# 查看特定令牌的 Key
GET rateLimit:token:5001
GET rateLimit:token:5001:success

# 查看 TTL
TTL rateLimit:token:5001
```

**预期结果：**
- 生成 Key：`rateLimit:token:5001` 和 `rateLimit:token:5001:success`
- TTL 为 duration * 2

---

### ✅ AC9: 输入验证生效

**测试步骤：**
1. 尝试创建令牌
2. 设置总请求数为 -1
3. RateLimitEnabled 为 false
4. 提交表单

**预期结果：**
- 后端返回错误：`总请求数必须在 0-10000 之间`

**测试脚本：**
```bash
curl -X POST http://localhost:3000/api/token \
  -H "Authorization: Bearer your-access-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test",
    "rate_limit_enabled": false,
    "rate_limit_total_count": -1,
    "rate_limit_success_count": 0,
    "rate_limit_duration": 1
  }'
```

---

### ✅ AC10: Redis Key 清理

**测试步骤：**
1. 创建令牌 ID 为 5001，启用独立限流
2. 使用该令牌请求（产生计数器）
3. 删除该令牌
4. 检查 Redis

**验证方式：**
```bash
# 删除前
redis-cli KEYS rateLimit:token:5001*

# 删除令牌
curl -X DELETE http://localhost:3000/api/token/5001 \
  -H "Authorization: Bearer your-access-token"

# 删除后（等待几秒让异步任务完成）
sleep 2
redis-cli KEYS rateLimit:token:5001*
```

**预期结果：**
- Redis 中的 Key 被清理
- 日志中显示：`Cleaned rate limit keys for token 5001 (deleted)`

---

### ✅ AC11: 配置无效时拒绝请求

**测试步骤：**
1. 通过数据库直接修改令牌配置为异常值
2. 使用该令牌请求

**测试脚本：**
```sql
-- 设置异常配置
UPDATE tokens SET
  rate_limit_enabled = 1,
  rate_limit_total_count = 0,
  rate_limit_success_count = 0,
  rate_limit_duration = 0
WHERE id = 1;
```

**预期结果：**
- 系统返回 500 错误
- 错误消息：`令牌限流配置异常，请联系管理员`
- 日志中记录：`Invalid token rate limit config`

---

### ✅ AC12: 并发请求计数准确

**测试步骤：**
1. 创建令牌，配置 10 次/分钟
2. 使用并发工具发送 100 个并发请求

**测试脚本：**
```bash
# 使用 Apache Bench
ab -n 100 -c 100 -H "Authorization: Bearer $TOKEN" \
  -p request.json -T application/json \
  http://localhost:3000/v1/chat/completions

# 或使用 wrk
wrk -t10 -c100 -d10s \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --script=post.lua \
  http://localhost:3000/v1/chat/completions
```

**预期结果：**
- 前 10 个请求通过
- 后续请求被拒绝
- 计数准确无误（检查 Redis）

---

### ✅ AC14: 权限控制生效

**测试步骤：**
1. 使用普通用户账号登录
2. 尝试创建令牌，设置限流为 5000 次/分钟
3. 提交表单

**预期结果：**
- 返回错误：`普通用户的令牌限流配置不能超过 1000 次/分钟`

---

### ✅ AC16: 状态变化时清理 Redis Key

**测试步骤：**
1. 创建令牌 ID 为 5001，启用独立限流
2. 使用该令牌请求（产生计数器）
3. 编辑令牌，将 RateLimitEnabled 改为 false
4. 检查 Redis

**预期结果：**
- Redis 中的 Key 被清理
- 日志中显示：`Cleaned rate limit keys for token 5001 (disabled)`

---

### ✅ AC17: Redis Key 自动过期

**测试步骤：**
1. 创建令牌，配置 10 次/1分钟
2. 使用该令牌请求
3. 检查 Redis Key 的 TTL

**验证方式：**
```bash
redis-cli TTL rateLimit:token:5001
```

**预期结果：**
- TTL 被设置为 2 分钟（duration * 2 = 120 秒）

---

### ✅ AC18: 前端成功请求数验证

**测试步骤：**
1. 在前端表单中设置：
   - 总请求数：100
   - 成功请求数：200
2. 提交表单

**预期结果：**
- 前端显示验证错误：`成功请求数不能超过总请求数`

---

## 压力测试

### 测试 1: 1000 QPS 并发请求

**工具：** wrk 或 Apache Bench

**测试脚本：**
```bash
# 使用 wrk
wrk -t10 -c1000 -d60s \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --script=post.lua \
  http://localhost:3000/v1/chat/completions
```

**验证：**
- 计数准确性
- 系统稳定性
- Redis 性能

---

### 测试 2: Redis Key 泄漏测试

**测试步骤：**
1. 创建 1000 个令牌，启用独立限流
2. 每个令牌请求 10 次
3. 删除所有令牌
4. 检查 Redis Key 数量

**验证方式：**
```bash
# 检查 Key 数量
redis-cli DBSIZE

# 检查限流 Key
redis-cli KEYS rateLimit:token:* | wc -l
```

**预期结果：**
- 所有 Key 被正确清理
- 没有孤儿 Key

---

## 日志检查

### 关键日志

1. **高配额警告：**
   ```
   High rate limit configured: token_id=1, count=5000
   ```

2. **配置异常：**
   ```
   Invalid token rate limit config: tokenId=1, total=0, success=0, duration=0
   ```

3. **Key 清理：**
   ```
   Cleaned rate limit keys for token 1 (deleted)
   Cleaned rate limit keys for token 1 (disabled)
   ```

4. **限流触发：**
   ```
   Rate limit check failed: ...
   Success rate limit exceeded for key: rateLimit:token:1:success
   ```

---

## 回归测试

确保现有功能不受影响：

1. ✅ 用户级别限流仍然正常工作
2. ✅ 分组级别限流仍然正常工作
3. ✅ 全局限流仍然正常工作
4. ✅ 未启用限流的令牌不受影响
5. ✅ 令牌的其他功能（额度、模型限制、IP 白名单）正常

---

## 测试报告模板

```markdown
## 测试报告

**测试日期：** YYYY-MM-DD
**测试人员：**
**测试环境：**
- Go 版本：
- Redis 版本：
- 数据库：

### 测试结果

| 测试用例 | 状态 | 备注 |
|---------|------|------|
| AC1: Token 模型支持新字段 | ✅ / ❌ | |
| AC2: 令牌级别限流生效 | ✅ / ❌ | |
| AC3: 令牌独立计数 | ✅ / ❌ | |
| ... | | |

### 发现的问题

1.
2.

### 建议

1.
2.
```
