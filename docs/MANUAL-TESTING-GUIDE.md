# 令牌级别限流功能测试指南

## 🎯 测试目标

验证令牌级别的独立限流功能是否正常工作。

## 📋 测试步骤

### 1. 创建测试 Token

1. 访问 http://localhost:3000
2. 使用管理员账号登录（stonefancyx / niuerWulian22.）
3. 进入「令牌」页面
4. 点击「新建令牌」
5. 配置限流参数：
   - **启用令牌级别限流**: 开启
   - **总请求数**: 10
   - **成功请求数**: 5
   - **时间窗口**: 60 秒
6. 保存并复制生成的 Token（格式：sk-xxxxx）

### 2. 手动测试限流

使用 curl 命令测试（替换 YOUR_TOKEN 为实际的 Token）：

```bash
# 测试脚本
TOKEN="sk-xxxxx"  # 替换为你的 Token

# 快速发送 15 个请求
for i in {1..15}; do
  echo "请求 $i:"
  curl -s http://localhost:3000/api/status \
    -H "Authorization: Bearer $TOKEN" \
    | jq -r '.success'
  sleep 0.5
done
```

**预期结果：**
- 前 10 个请求应该成功（返回 true）
- 第 11-15 个请求应该被限流（返回 false 或错误信息）

### 3. 查看 Redis 中的限流 Key

```bash
# 查看所有限流 Key
docker exec redis-standalone redis-cli KEYS "rateLimit:*"

# 查看特定 Token 的限流计数
docker exec redis-standalone redis-cli GET "rateLimit:token:1"

# 查看 Key 的过期时间（TTL）
docker exec redis-standalone redis-cli TTL "rateLimit:token:1"
```

**预期结果：**
- 应该看到 `rateLimit:token:X` 格式的 Key
- GET 命令应该返回当前请求计数
- TTL 应该显示剩余过期时间（秒）

### 4. 测试限流重置

等待 60 秒后，限流计数器应该自动重置：

```bash
# 等待 60 秒
sleep 60

# 再次发送请求
curl -s http://localhost:3000/api/status \
  -H "Authorization: Bearer $TOKEN" \
  | jq -r '.success'
```

**预期结果：**
- 请求应该再次成功
- Redis 中的计数器应该重置为 1

### 5. 测试前端 UI

1. 在「令牌」页面点击「编辑」
2. 验证限流配置正确显示
3. 修改限流参数并保存
4. 验证修改后的限流规则生效

## 🔍 验证要点

### ✅ 功能验证

- [ ] 限流开关可以正常开启/关闭
- [ ] 总请求数限制生效
- [ ] 成功请求数限制生效
- [ ] 时间窗口正确
- [ ] 超过限制后返回 429 错误
- [ ] 限流计数器自动过期

### ✅ Redis Key 验证

- [ ] Key 格式正确：`rateLimit:token:{token_id}`
- [ ] Key 有正确的 TTL（时间窗口 * 2）
- [ ] 计数器递增正确
- [ ] 达到限制后不再递增

### ✅ 边界情况

- [ ] 总请求数 = 0 时不限流
- [ ] 成功请求数 > 总请求数时验证失败
- [ ] 禁用限流后不创建 Redis Key
- [ ] 删除 Token 后清理 Redis Key

## 🐛 常见问题

### Q1: 限流不生效？

**检查：**
```bash
# 1. 确认 Redis 连接
docker ps | grep redis

# 2. 查看后端日志
# 应该看到 "Redis is enabled"

# 3. 检查 Token 配置
# 确认 rate_limit_enabled = true
```

### Q2: Redis Key 没有创建？

**原因：**
- 限流未开启
- Token 配置错误
- Redis 连接失败

**解决：**
```bash
# 检查 Redis 连接
docker exec redis-standalone redis-cli ping
# 应该返回: PONG
```

### Q3: 样式显示异常？

**解决：**
```bash
# 重新构建前端
cd web
bun run build

# 刷新浏览器（Ctrl+Shift+R 强制刷新）
```

## 📊 测试数据示例

### 测试场景 1：基本限流

```
配置：
- 总请求数: 10
- 成功请求数: 5
- 时间窗口: 60秒

预期：
- 前 10 个请求成功
- 第 11 个请求被限流
- 60 秒后限流重置
```

### 测试场景 2：成功请求数限制

```
配置：
- 总请求数: 20
- 成功请求数: 10
- 时间窗口: 60秒

预期：
- 前 10 个成功请求通过
- 第 11-20 个请求可能通过（如果是失败请求）
- 第 11 个成功请求被限流
```

### 测试场景 3：快速请求

```
配置：
- 总请求数: 5
- 成功请求数: 3
- 时间窗口: 10秒

测试：
- 1 秒内发送 10 个请求

预期：
- 前 5 个请求成功
- 后 5 个请求被限流
- 10 秒后可以再次请求
```

## ✅ 测试完成标准

所有以下项目都通过：

1. ✅ 前端 UI 正常显示限流配置
2. ✅ 限流规则正确生效
3. ✅ Redis Key 正确创建和清理
4. ✅ 超过限制返回正确的错误信息
5. ✅ 时间窗口过期后自动重置
6. ✅ 修改配置后立即生效
7. ✅ 删除 Token 后清理 Redis Key

---

**测试完成后，请在浏览器中验证样式是否恢复正常！**
