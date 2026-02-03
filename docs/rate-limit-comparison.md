# 请求速率限制模型 - 改造前后对比

## 一、架构对比图

### 改造前架构

```
┌─────────────────────────────────────────────────────────────┐
│                         请求入口                              │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                    获取令牌分组信息                           │
│                  (TokenGroup / UserGroup)                    │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
              ┌───────────┴───────────┐
              │                       │
              ▼                       ▼
    ┌─────────────────┐     ┌─────────────────┐
    │  找到分组配置    │     │  使用全局默认    │
    │  使用分组限制    │     │  使用全局限制    │
    └────────┬─────────┘     └────────┬─────────┘
             │                        │
             └───────────┬────────────┘
                         │
                         ▼
            ┌────────────────────────┐
            │   生成限流 Key         │
            │  rateLimit:{userId}    │
            │  (所有令牌共享计数器)   │
            └────────────┬───────────┘
                         │
                         ▼
            ┌────────────────────────┐
            │   Redis 计数检查       │
            │   INCR + EXPIRE        │
            └────────────┬───────────┘
                         │
              ┌──────────┴──────────┐
              │                     │
              ▼                     ▼
        ┌─────────┐           ┌─────────┐
        │ 通过    │           │ 拒绝    │
        │ 继续    │           │ 429     │
        └─────────┘           └─────────┘
```

**问题：**
- ❌ 用户的所有令牌共享同一个计数器
- ❌ 无法为不同令牌设置不同的速率限制
- ❌ 一个令牌耗尽配额会影响其他令牌

---

### 改造后架构

```
┌─────────────────────────────────────────────────────────────┐
│                         请求入口                              │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│              检查令牌是否启用独立限流                          │
│              (Token.RateLimitEnabled)                        │
└─────────────────────────┬───────────────────────────────────┘
                          │
              ┌───────────┴───────────┐
              │                       │
              ▼                       ▼
    ┌─────────────────┐     ┌─────────────────┐
    │  令牌独立限流    │     │  分组/全局限流   │
    │  (优先级最高)    │     │  (向后兼容)      │
    └────────┬─────────┘     └────────┬─────────┘
             │                        │
             ▼                        ▼
┌────────────────────────┐ ┌────────────────────────┐
│ 使用令牌配置:          │ │ 获取令牌分组:          │
│ • RateLimitCount       │ │ • 找到分组 → 分组配置  │
│ • RateLimitDuration    │ │ • 未找到 → 全局默认    │
└────────┬───────────────┘ └────────┬───────────────┘
         │                          │
         ▼                          ▼
┌────────────────────────┐ ┌────────────────────────┐
│ 生成限流 Key:          │ │ 生成限流 Key:          │
│ rateLimit:token:{id}   │ │ rateLimit:user:{id}    │
│ (令牌独立计数器) ✅    │ │ (用户共享计数器)       │
└────────┬───────────────┘ └────────┬───────────────┘
         │                          │
         └──────────┬───────────────┘
                    │
                    ▼
       ┌────────────────────────┐
       │   Redis 计数检查       │
       │   INCR + EXPIRE        │
       └────────────┬───────────┘
                    │
         ┌──────────┴──────────┐
         │                     │
         ▼                     ▼
   ┌─────────┐           ┌─────────┐
   │ 通过    │           │ 拒绝    │
   │ 继续    │           │ 429     │
   └─────────┘           └─────────┘
```

**优势：**
- ✅ 支持令牌级别独立限流
- ✅ 向后兼容现有分组/全局限流
- ✅ 灵活的三级优先级配置

---

## 二、数据模型对比

### Token 表结构变化

```diff
  CREATE TABLE tokens (
      id INTEGER PRIMARY KEY,
      user_id INTEGER NOT NULL,
      key TEXT NOT NULL,
      name TEXT,
      group_name TEXT,
      quota INTEGER DEFAULT 0,
      status INTEGER DEFAULT 1,
+     rate_limit_enabled BOOLEAN DEFAULT FALSE,
+     rate_limit_count INTEGER DEFAULT 0,
+     rate_limit_duration INTEGER DEFAULT 0,
      created_time BIGINT,
      accessed_time BIGINT,
      expired_time BIGINT
  );
```

**新增字段说明：**

| 字段 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `rate_limit_enabled` | BOOLEAN | 是否启用令牌级别限流 | `false` |
| `rate_limit_count` | INTEGER | 每周期最大请求数 | `0` |
| `rate_limit_duration` | INTEGER | 限流周期（分钟） | `0` |

---

## 三、限流 Key 对比

### 改造前

```
所有场景统一使用:
┌──────────────────────────┐
│  rateLimit:{userId}      │
│                          │
│  用户的所有令牌共享      │
│  同一个 Redis 计数器     │
└──────────────────────────┘
```

### 改造后

```
根据配置动态选择:

令牌启用独立限流:
┌──────────────────────────┐
│  rateLimit:token:{tokenId}│
│                          │
│  每个令牌独立计数器      │
└──────────────────────────┘

令牌未启用独立限流:
┌──────────────────────────┐
│  rateLimit:user:{userId} │
│                          │
│  用户的令牌共享计数器    │
└──────────────────────────┘
```

---

## 四、配置优先级对比

### 改造前（2级）

```
┌─────────────────────────────────────┐
│  优先级 1: 分组配置                  │
│  • 查找 TokenGroup 或 UserGroup     │
│  • 使用分组的限流配置                │
└─────────────────┬───────────────────┘
                  │ 未找到
                  ▼
┌─────────────────────────────────────┐
│  优先级 2: 全局默认                  │
│  • ModelRequestRateLimitCount       │
│  • ModelRequestRateLimitDuration    │
└─────────────────────────────────────┘
```

### 改造后（3级）

```
┌─────────────────────────────────────┐
│  优先级 1: 令牌配置 (NEW!)           │
│  • Token.RateLimitEnabled = true    │
│  • Token.RateLimitCount             │
│  • Token.RateLimitDuration          │
└─────────────────┬───────────────────┘
                  │ 未启用
                  ▼
┌─────────────────────────────────────┐
│  优先级 2: 分组配置                  │
│  • 查找 TokenGroup 或 UserGroup     │
│  • 使用分组的限流配置                │
└─────────────────┬───────────────────┘
                  │ 未找到
                  ▼
┌─────────────────────────────────────┐
│  优先级 3: 全局默认                  │
│  • ModelRequestRateLimitCount       │
│  • ModelRequestRateLimitDuration    │
└─────────────────────────────────────┘
```

---

## 五、实际场景对比

### 场景：用户有 3 个令牌，不同用途

```
用户 ID: 1001
├─ Token A (ID: 5001) - 生产环境 API
├─ Token B (ID: 5002) - 测试环境 API
└─ Token C (ID: 5003) - 开发环境 API
```

#### 改造前行为

```
全局配置: 60 次/分钟
分组配置: premium 组 = 120 次/分钟

用户将 3 个令牌都加入 premium 组

┌─────────────────────────────────────────┐
│  Redis Key: rateLimit:1001              │
│  限制: 120 次/分钟                       │
│  计数器: 共享                            │
└─────────────────────────────────────────┘

时间线:
00:00 - Token A 请求 80 次 → 计数器 = 80
00:30 - Token B 请求 30 次 → 计数器 = 110
00:45 - Token C 请求 20 次 → ❌ 被拒绝 (110+20 > 120)
01:00 - 计数器重置

问题: Token C 被 A 和 B 的请求影响
```

#### 改造后行为

```
配置:
• Token A: 启用独立限流, 100 次/分钟
• Token B: 启用独立限流, 50 次/分钟
• Token C: 不启用, 使用分组配置 120 次/分钟

┌─────────────────────────────────────────┐
│  Token A                                │
│  Redis Key: rateLimit:token:5001       │
│  限制: 100 次/分钟                       │
│  计数器: 独立                            │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│  Token B                                │
│  Redis Key: rateLimit:token:5002       │
│  限制: 50 次/分钟                        │
│  计数器: 独立                            │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│  Token C                                │
│  Redis Key: rateLimit:user:1001        │
│  限制: 120 次/分钟                       │
│  计数器: 用户级别                        │
└─────────────────────────────────────────┘

时间线:
00:00 - Token A 请求 80 次 → A 计数器 = 80 ✅
00:30 - Token B 请求 30 次 → B 计数器 = 30 ✅
00:45 - Token C 请求 20 次 → C 计数器 = 20 ✅
00:50 - Token A 请求 20 次 → A 计数器 = 100 ✅
00:55 - Token B 请求 20 次 → B 计数器 = 50 ✅
01:00 - 所有计数器重置

优势: 每个令牌独立计数，互不影响
```

---

## 六、代码关键变化

### 中间件逻辑对比

#### 改造前 (`middleware/model-rate-limit.go`)

```go
func ModelRequestRateLimit() func(c *gin.Context) {
    return func(c *gin.Context) {
        // 1. 获取全局默认配置
        duration := int64(setting.ModelRequestRateLimitDurationMinutes * 60)
        maxCount := setting.ModelRequestRateLimitCount

        // 2. 尝试获取分组配置
        group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
        groupCount, _, found := setting.GetGroupRateLimit(group)
        if found {
            maxCount = groupCount
        }

        // 3. 生成限流 Key (基于 userId)
        userId := strconv.Itoa(c.GetInt("id"))
        limitKey := fmt.Sprintf("rateLimit:%s", userId)

        // 4. Redis 计数检查
        count, err := common.RedisIncrWithExpire(limitKey, duration)
        if err != nil {
            // 错误处理
        }

        if count > maxCount {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "请求过于频繁",
            })
            c.Abort()
            return
        }

        c.Next()
    }
}
```

#### 改造后

```go
func ModelRequestRateLimit() func(c *gin.Context) {
    return func(c *gin.Context) {
        userId := c.GetInt("id")
        tokenId := c.GetInt("token_id")

        var maxCount int
        var duration int64
        var limitKey string

        // 1. 优先检查令牌级别配置 (NEW!)
        tokenRateLimitEnabled := common.GetContextKeyBool(c,
            constant.ContextKeyTokenRateLimitEnabled)

        if tokenRateLimitEnabled {
            // 使用令牌独立配置
            maxCount = common.GetContextKeyInt(c,
                constant.ContextKeyTokenRateLimitCount)
            duration = int64(common.GetContextKeyInt(c,
                constant.ContextKeyTokenRateLimitDuration) * 60)
            limitKey = fmt.Sprintf("rateLimit:token:%d", tokenId)
        } else {
            // 2. 回退到分组/全局配置
            group := common.GetContextKeyString(c,
                constant.ContextKeyTokenGroup)
            groupCount, _, found := setting.GetGroupRateLimit(group)

            if found {
                maxCount = groupCount
            } else {
                maxCount = setting.ModelRequestRateLimitCount
            }

            duration = int64(setting.ModelRequestRateLimitDurationMinutes * 60)
            limitKey = fmt.Sprintf("rateLimit:user:%d", userId)
        }

        // 3. Redis 计数检查 (逻辑相同)
        count, err := common.RedisIncrWithExpire(limitKey, duration)
        if err != nil {
            // 错误处理
        }

        if count > maxCount {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "请求过于频繁",
                "limit": maxCount,
                "window": duration / 60,
            })
            c.Abort()
            return
        }

        c.Next()
    }
}
```

**关键变化：**
1. ✅ 新增令牌级别配置检查
2. ✅ 动态生成限流 Key（token 或 user）
3. ✅ 保持向后兼容性

---

## 七、API 变化

### 新增/修改的 API 端点

#### 1. 创建令牌（新增字段）

```http
POST /api/token

Request:
{
  "name": "Production API Key",
  "group": "premium",
  "rate_limit_enabled": true,      // NEW
  "rate_limit_count": 100,         // NEW
  "rate_limit_duration": 1         // NEW (分钟)
}

Response:
{
  "success": true,
  "data": {
    "id": 5001,
    "key": "sk-xxx",
    "rate_limit_enabled": true,
    "rate_limit_count": 100,
    "rate_limit_duration": 1
  }
}
```

#### 2. 更新令牌（新增字段）

```http
PUT /api/token/:id

Request:
{
  "rate_limit_enabled": true,
  "rate_limit_count": 200,
  "rate_limit_duration": 5
}
```

#### 3. 查询令牌（返回新字段）

```http
GET /api/token/:id

Response:
{
  "id": 5001,
  "name": "Production API Key",
  "group": "premium",
  "rate_limit_enabled": true,      // NEW
  "rate_limit_count": 100,         // NEW
  "rate_limit_duration": 1,        // NEW
  "created_time": 1234567890
}
```

---

## 八、使用场景对比表

| 使用场景 | 改造前 | 改造后 |
|----------|--------|--------|
| 为不同客户分配不同速率的 API Key | ❌ 只能通过分组，同组共享 | ✅ 每个令牌独立配置 |
| 同一用户不同用途的令牌限制不同 | ❌ 无法实现 | ✅ 支持 |
| 临时给某个令牌提高/降低限制 | ❌ 需要修改分组配置，影响其他令牌 | ✅ 直接修改令牌配置 |
| 保持现有分组限流行为 | ✅ 默认行为 | ✅ 令牌不启用时自动回退 |
| 按用户统一限流 | ✅ 默认行为 | ✅ 令牌不启用时保持 |
| 精细化控制每个 API Key | ❌ 不支持 | ✅ 完全支持 |

---

## 九、迁移兼容性

### 现有系统迁移

```
改造前的系统:
• 所有令牌的 rate_limit_enabled = false (默认值)
• 自动使用分组/全局配置
• 行为完全一致 ✅

改造后的系统:
• 默认不启用令牌级别限流
• 向后兼容现有配置
• 可逐步迁移到令牌级别限流
```

### 迁移步骤

```
1. 部署新版本
   ↓
2. 现有令牌继续使用分组/全局配置 (rate_limit_enabled = false)
   ↓
3. 根据需要，逐个令牌启用独立限流
   ↓
4. 完全迁移（可选）
```

---

## 十、性能影响

| 指标 | 改造前 | 改造后 | 影响 |
|------|--------|--------|------|
| Redis Key 数量 | 每用户 1 个 | 每令牌 1 个（启用时） | ⚠️ 增加 |
| Redis 操作次数 | 相同 | 相同 | ✅ 无影响 |
| 内存占用 | 低 | 中等 | ⚠️ 略增 |
| 查询性能 | 相同 | 相同 | ✅ 无影响 |
| 灵活性 | 低 | 高 | ✅ 提升 |

**建议：**
- 仅对需要独立限流的令牌启用该功能
- 大部分令牌可继续使用分组/全局配置
- 定期清理过期的 Redis Key

---

## 总结

### 改造前的限制
- ❌ 用户级别限流，所有令牌共享配额
- ❌ 无法为不同令牌设置不同速率
- ❌ 灵活性差，难以满足多样化需求

### 改造后的优势
- ✅ 支持令牌级别独立限流
- ✅ 三级优先级配置（令牌 > 分组 > 全局）
- ✅ 完全向后兼容
- ✅ 灵活满足各种业务场景
- ✅ 可逐步迁移，无需一次性改造

### 适用场景
- 🎯 SaaS 平台为不同客户提供差异化服务
- 🎯 同一用户的生产/测试/开发环境隔离
- 🎯 临时调整某个 API Key 的速率限制
- 🎯 精细化的流量控制和成本管理
