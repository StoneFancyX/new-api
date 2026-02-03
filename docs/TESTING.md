# 令牌级别速率限制 - 快速测试指南

## 🚀 快速开始

### 1. 启动项目

```bash
# 启动 Redis（如果还没启动）
redis-server

# 启动后端
go run main.go

# 新开终端，启动前端
cd web && bun run dev
```

### 2. 前端测试（5分钟）

1. 打开浏览器访问 http://localhost:3000
2. 登录系统
3. 进入"令牌管理"页面
4. 点击"创建令牌"或"编辑令牌"
5. **检查点：** 是否看到新的"速率限制"配置区域？
6. 启用"令牌级别限流"开关
7. **检查点：** 配置字段是否正确显示？
8. 点击快捷设置按钮（1分钟/60次）
9. **检查点：** 字段是否自动填充？
10. 提交表单
11. **检查点：** 是否保存成功？

✅ 如果以上都通过，前端功能正常！

---

### 3. API 限流测试（5分钟）

#### 测试 1: 基本限流功能

```bash
# 1. 创建一个令牌，配置：10次/1分钟
# 2. 复制令牌 key（如：sk-xxx）
# 3. 运行测试脚本

./bin/rate_limit_test.sh localhost:3000 sk-your-token-here 15
```

**预期结果：**
- 前 10 次请求：✅ Success (200)
- 后 5 次请求：🚫 Rate Limited (429)
- 错误消息：`您已达到总请求数限制：1分钟内最多请求10次`

---

#### 测试 2: 令牌独立计数

```bash
# 1. 创建令牌 A，启用限流：10次/1分钟
# 2. 创建令牌 B，不启用限流

# 测试令牌 A
./bin/rate_limit_test.sh localhost:3000 sk-token-a 12

# 测试令牌 B（应该不受影响）
./bin/rate_limit_test.sh localhost:3000 sk-token-b 12
```

**预期结果：**
- 令牌 A：前 10 次成功，后 2 次被限流
- 令牌 B：全部成功（使用全局配置）

---

#### 测试 3: Redis Key 检查

```bash
# 1. 使用令牌请求几次
./bin/rate_limit_test.sh localhost:3000 sk-your-token 5

# 2. 检查 Redis
redis-cli

# 3. 在 Redis CLI 中执行
KEYS rateLimit:token:*
GET rateLimit:token:1
GET rateLimit:token:1:success
TTL rateLimit:token:1
```

**预期结果：**
- 看到 `rateLimit:token:1` 和 `rateLimit:token:1:success`
- 值为请求次数（如：5）
- TTL 为 120 秒（duration * 2）

---

#### 测试 4: Redis Key 清理

```bash
# 1. 使用令牌请求几次（产生计数器）
./bin/rate_limit_test.sh localhost:3000 sk-your-token 5

# 2. 在前端删除该令牌

# 3. 等待 2 秒（异步清理）
sleep 2

# 4. 检查 Redis
redis-cli KEYS rateLimit:token:*
```

**预期结果：**
- Redis 中的 Key 被清理
- 后端日志显示：`Cleaned rate limit keys for token X (deleted)`

---

### 4. 输入验证测试（3分钟）

#### 前端验证

1. 创建令牌，启用限流
2. 设置：
   - 总请求数：100
   - 成功请求数：200（故意超过总数）
3. 提交表单

**预期结果：**
- 前端显示错误：`成功请求数不能超过总请求数`

#### 后端验证

```bash
# 使用 curl 测试
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

**预期结果：**
- 返回错误：`总请求数必须在 0-10000 之间`

---

## 🎯 核心测试清单

快速验证核心功能：

- [ ] 前端显示速率限制配置区域
- [ ] 快捷设置按钮工作正常
- [ ] 表单验证正确（成功请求数 ≤ 总请求数）
- [ ] 限流功能生效（第 N+1 次请求被拒绝）
- [ ] 令牌独立计数（不同令牌互不影响）
- [ ] Redis Key 正确生成
- [ ] Redis Key 正确清理（删除令牌时）
- [ ] 向后兼容（未启用限流的令牌正常工作）

---

## 🐛 常见问题

### Q1: 前端看不到速率限制配置？
**A:** 检查前端是否重新编译：
```bash
cd web && bun run dev
```

### Q2: 限流不生效？
**A:** 检查：
1. Redis 是否启动：`redis-cli ping`
2. 后端日志是否有错误
3. 令牌是否启用了独立限流

### Q3: Redis Key 没有清理？
**A:** 清理是异步的，等待 2-3 秒后再检查

### Q4: 测试脚本报错？
**A:** 确保：
1. 脚本有执行权限：`chmod +x bin/rate_limit_test.sh`
2. 已安装 curl 和 bc

---

## 📊 性能测试（可选）

如果需要测试并发性能：

```bash
# 使用 Apache Bench
ab -n 100 -c 10 \
  -H "Authorization: Bearer sk-your-token" \
  -p request.json -T application/json \
  http://localhost:3000/v1/chat/completions

# request.json 内容：
# {"messages":[{"content":"hi","role":"user"}],"model":"gpt-3.5-turbo","max_tokens":1}
```

---

## ✅ 测试完成

如果以上测试都通过，说明功能实现正确！

可以提交代码了：
```bash
git add .
git commit -m "feat: 实现令牌级别独立速率限制"
git push origin feature/token-level-rate-limit
```
