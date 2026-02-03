---
title: '令牌级别独立速率限制'
slug: 'token-level-rate-limit'
created: '2025-02-02'
status: 'ready-for-dev'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go 1.18+', 'Gin', 'GORM v2', 'Redis', 'React 18+', 'Semi-UI', 'Vite', 'react-i18next']
files_to_modify:
  - 'model/token.go'
  - 'middleware/model-rate-limit.go'
  - 'controller/token.go'
  - 'constant/context_key.go'
  - 'middleware/auth.go'
  - 'web/src/components/table/tokens/modals/EditTokenModal.jsx'
code_patterns: ['GORM模型', 'Gin中间件', 'Redis限流', 'Context传值', 'Semi-UI表单', 'i18n翻译']
test_patterns: ['手动测试为主', '无自动化测试框架', '使用channel-test.go作为参考模式']
---

# Tech-Spec: 令牌级别独立速率限制

**Created:** 2025-02-02

## Overview

### Problem Statement

当前系统的速率限制是基于用户级别的，同一用户的所有令牌共享同一个 Redis 计数器（`rateLimit:{userId}`）。这导致：

1. **无法为不同令牌设置不同的速率限制** - 例如生产环境令牌需要更高的速率，测试环境令牌需要较低的速率
2. **一个令牌耗尽配额会影响其他令牌** - 如果一个令牌请求过多，会导致同一用户的其他令牌也被限流
3. **缺乏灵活性** - 无法针对特定客户或用途进行精细化的流量控制

### Solution

在 Token 模型中新增 4 个字段，支持令牌级别的独立速率限制配置：

- `RateLimitEnabled` - 是否启用令牌级别限流
- `RateLimitTotalCount` - 每周期最大总请求数
- `RateLimitSuccessCount` - 每周期最大成功请求数
- `RateLimitDuration` - 限流周期（分钟）

修改限流中间件，实现三级优先级配置：
1. **令牌配置**（最高优先级）- 如果令牌启用独立限流，使用令牌配置
2. **分组配置** - 如果令牌未启用独立限流，使用分组配置
3. **全局默认** - 如果没有分组配置，使用全局默认

### Scope

**In Scope:**

1. ✅ Token 模型新增 4 个速率限制字段
2. ✅ 数据库迁移（通过 GORM AutoMigrate）
3. ✅ 修改限流中间件，支持令牌级别限流
4. ✅ 修改认证中间件，将令牌限流配置写入 Context
5. ✅ 修改 Token Controller，支持创建/更新/查询新字段
6. ✅ 修改前端令牌编辑页面，添加速率限制配置表单
7. ✅ 完全向后兼容现有系统
8. ✅ 输入验证和防御性编程
9. ✅ Redis Key 清理机制
10. ✅ 故障模式预防措施

**Out of Scope:**

1. ❌ 修改全局速率限制配置页面
2. ❌ 修改分组速率限制配置
3. ❌ 速率限制的统计和监控功能（建议未来实现）
4. ❌ 速率限制的历史记录
5. ❌ 审计日志系统（建议未来实现）
6. ❌ 基于角色的访问控制（RBAC）

## Context for Development

### Codebase Patterns

**1. GORM 模型定义模式**
- 模型定义在 `model/` 目录
- 使用 GORM 标签定义字段属性：`gorm:"default:false"`
- 使用 JSON 标签定义 API 返回字段名：`json:"rate_limit_enabled"`
- Update 方法需要在 Select 中显式指定要更新的字段
- 示例：`model/token.go` 中的 Token struct 有 31 个字段
- Update() 方法使用 `DB.Model(token).Select(...).Updates(token)` 模式

**2. Gin 中间件模式**
- 中间件定义在 `middleware/` 目录
- 使用 `c.Set()` 和 `c.Get()` 在中间件间传递数据
- 使用 `common.GetContextKeyString()` 等辅助函数获取 Context 值（定义在 `common/gin.go`）
- 限流逻辑分为 Redis 和内存两种实现
- 中间件执行顺序：`TokenAuth()` → `ModelRequestRateLimit()` → 业务逻辑
- 当前限流中间件使用 `userId` 作为限流 Key：`fmt.Sprintf("rateLimit:%s", userId)`

**3. Context Key 常量模式**
- Context Key 定义在 `constant/context_key.go`
- 使用 `type ContextKey string` 定义类型
- 使用字符串常量避免硬编码
- 命名规范：`ContextKeyTokenXxx`（如 `ContextKeyTokenGroup`、`ContextKeyTokenModelLimit`）
- 通过 `common.SetContextKey()` 和 `common.GetContextKey()` 访问

**4. Controller 模式**
- Controller 定义在 `controller/` 目录
- 使用 `c.ShouldBindJSON()` 绑定请求体
- 创建/更新时需要手动构建 `cleanToken` 对象，避免注入攻击
- 使用 `common.ApiSuccess()` 和 `common.ApiError()` 统一返回格式
- 权限检查：使用 `model.GetTokenByIds(tokenId, userId)` 确保用户只能操作自己的令牌
- 输入验证：在 Controller 层进行业务逻辑验证（如名称长度、过期时间、额度等）

**5. 前端表单模式**
- 使用 Semi-UI 的 Form 组件
- 使用 `formApiRef` 引用表单 API
- 表单字段使用 `field` 属性绑定数据
- 使用 `getInitValues()` 函数设置初始值
- 使用 `rules` 属性定义验证规则
- 使用 `extraText` 属性显示帮助文本
- 使用 `showClear` 属性显示清除按钮
- 使用 `useTranslation()` hook 进行国际化
- 表单布局使用 `Card` + `Row` + `Col` 组件

**6. 认证中间件模式**
- `middleware/auth.go` 中的 `TokenAuth()` 负责令牌验证
- 验证通过后调用 `SetupContextForToken()` 将令牌信息写入 Context
- 当前已写入的令牌信息：`token_id`、`token_key`、`token_name`、`token_unlimited_quota`、`token_model_limit_enabled`、`token_model_limit`、`token_group`、`token_cross_group_retry`
- 需要在此处添加新的速率限制字段

**7. 限流中间件模式**
- `middleware/model-rate-limit.go` 中的 `ModelRequestRateLimit()` 负责限流
- 支持 Redis 和内存两种实现
- Redis 实现使用令牌桶算法（`common/limiter` 包）
- 内存实现使用固定时间窗口算法
- 当前优先级：分组配置 > 全局配置
- 需要添加：令牌配置 > 分组配置 > 全局配置

**8. Redis Key 命名模式**
- 当前用户级别限流 Key：`rateLimit:{userId}` 和 `rateLimit:MRRLS:{userId}`
- 需要添加令牌级别限流 Key：`rateLimit:token:{tokenId}` 和 `rateLimit:token:{tokenId}:success`
- 使用 `fmt.Sprintf()` 生成 Key，确保类型安全

### Files to Reference

| File | Purpose | Key Findings |
| ---- | ------- | ------------ |
| `model/token.go` | Token 模型定义，需要新增 4 个字段 | 31 个现有字段，Update() 方法使用 Select 显式指定字段，Delete() 方法有 Redis 缓存清理逻辑 |
| `middleware/model-rate-limit.go` | 速率限制中间件，需要修改限流逻辑 | 201 行代码，支持 Redis 和内存两种实现，使用 `userId` 作为限流 Key，区分总请求数和成功请求数 |
| `middleware/auth.go` | 认证中间件，需要将令牌限流配置写入 Context | 331 行代码，`SetupContextForToken()` 函数（301-330行）负责写入令牌信息到 Context |
| `controller/token.go` | Token Controller，需要支持新字段的创建/更新/查询 | 292 行代码，`AddToken()` 和 `UpdateToken()` 使用 cleanToken 模式防止注入，包含输入验证逻辑 |
| `constant/context_key.go` | Context Key 常量定义，需要新增 4 个常量 | 58 行代码，使用 `type ContextKey string`，已有 22 个令牌相关的 Context Key |
| `web/src/components/table/tokens/modals/EditTokenModal.jsx` | 令牌编辑模态框，需要添加速率限制表单字段 | 580 行代码，使用 Semi-UI Form 组件，分为基本信息、额度设置、访问限制三个 Card 区域 |
| `common/gin.go` | Gin Context 辅助函数 | 提供 `GetContextKeyString()`、`GetContextKeyInt()`、`GetContextKeyBool()` 等辅助函数 |
| `docs/rate-limit-comparison.md` | 速率限制对比文档，包含详细的改造前后对比 | 详细说明了当前问题和解决方案，包含代码示例和配置说明 |

### Technical Decisions

**1. 字段命名**
- 使用 `rate_limit_` 前缀保持一致性
- 区分 `total_count` 和 `success_count` 两种计数
- 使用 `duration` 表示周期，单位为分钟

**2. 优先级设计**
- 令牌配置 > 分组配置 > 全局默认
- 只有当 `RateLimitEnabled = true` 时才使用令牌配置
- 保持向后兼容，默认 `RateLimitEnabled = false`

**3. Redis Key 设计**
- 令牌级别：`rateLimit:token:{tokenId}` 和 `rateLimit:token:{tokenId}:success`
- 用户级别：`rateLimit:user:{userId}` 和 `rateLimit:user:{userId}:success`
- 保持与现有 Key 格式的一致性

**4. 数据库迁移**
- 使用 GORM 的 AutoMigrate 自动迁移
- 新字段设置默认值，确保现有数据兼容
- 不需要手动编写 SQL 迁移脚本

**5. 前端表单设计**
- 使用 Switch 控制是否启用令牌级别限流
- 只有启用时才显示配置字段
- 提供快捷设置按钮（如 1分钟/5分钟/10分钟）

**6. Context 传值方式**
- 在 `middleware/auth.go` 的 `SetupContextForToken()` 函数中写入令牌配置
- 在 `middleware/model-rate-limit.go` 的 `ModelRequestRateLimit()` 函数中读取配置
- 使用 `common.SetContextKey()` 和 `common.GetContextKeyInt()` 等辅助函数
- 避免重复查询数据库，提升性能

**7. 中间件执行顺序**
- 路由配置中已明确定义：`TokenAuth()` → `ModelRequestRateLimit()` → 业务逻辑
- 认证中间件先执行，将令牌信息写入 Context
- 限流中间件后执行，从 Context 读取令牌配置

**8. 输入验证策略**
- 前端验证：使用 Semi-UI Form 的 `rules` 属性
- 后端验证：在 Controller 层进行业务逻辑验证
- 验证规则：
  - `rate_limit_total_count`: 1-10000
  - `rate_limit_success_count`: 1-rate_limit_total_count
  - `rate_limit_duration`: 1-1440（分钟）

**9. 防御性编程**
- 在限流中间件中检查配置有效性
- 如果配置无效（值为 0 或负数），记录日志并回退到分组/全局配置
- 确保系统在异常情况下仍能正常运行

**10. Redis Key 清理**
- 在 `model/token.go` 的 `Delete()` 方法中添加清理逻辑
- 使用 `gopool.Go()` 异步清理，避免阻塞主流程
- 清理两个 Key：`rateLimit:token:{tokenId}` 和 `rateLimit:token:{tokenId}:success`

---

## 架构决策记录 (ADR)

### ADR-1: Redis Key 命名策略

**决策：** 使用完整 Key 名 `rateLimit:token:{tokenId}`

**备选方案：**
- 短 Key 名：`rateLimit:t:{tokenId}` - 节省内存但可读性差
- 分层 Key 名：`rateLimit:v2:token:{tokenId}:total` - 扩展性好但过于复杂

**理由：**
1. 与现有 `rateLimit:user:{userId}` 保持一致，降低学习成本
2. 可读性对运维和调试至关重要
3. 内存占用差异在实际场景中可忽略（10000 个令牌仅多占用 100KB）
4. 向后兼容性最好

**权衡：**
- ✅ 可读性：⭐⭐⭐⭐⭐
- ✅ 兼容性：⭐⭐⭐⭐⭐
- ⚠️ 内存占用：⭐⭐⭐（可接受）

---

### ADR-2: 限流算法选择

**决策：** 保持固定时间窗口算法

**备选方案：**
- 滑动时间窗口：更精确但性能开销大
- 令牌桶算法：允许突发流量但实现复杂

**理由：**
1. 与现有实现保持一致，降低改造风险
2. 性能最优（单次 Redis INCR 操作），适合高并发场景
3. 对于大多数业务场景，固定窗口已足够
4. 未来可以通过配置切换到其他算法（预留扩展点）

**局限性：**
- 存在时间窗口边界问题：用户可能在窗口末尾请求 N 次，窗口开始时再请求 N 次
- 如果需要更精确的限流，可以考虑滑动窗口或令牌桶算法

**权衡：**
- ✅ 性能：⭐⭐⭐⭐⭐
- ✅ 实现复杂度：⭐⭐⭐⭐⭐
- ⚠️ 精确度：⭐⭐⭐（可接受）

---

### ADR-3: 配置获取方式

**决策：** 通过 Context 传值

**备选方案：**
- 限流中间件独立查询数据库/缓存
- 使用专门的配置缓存层

**理由：**
1. 认证中间件已经查询了 Token，复用数据避免重复查询
2. 性能最优，不增加额外的数据库/缓存查询
3. 实现简单，与现有代码模式一致（如 `ContextKeyTokenGroup`）
4. 中间件执行顺序在路由配置中已明确定义

**风险缓解：**
- 在限流中间件中添加防御性检查，如果 Context 中没有值，记录错误日志
- 在路由配置中明确注释中间件的执行顺序

**权衡：**
- ✅ 性能：⭐⭐⭐⭐⭐
- ✅ 实现复杂度：⭐⭐⭐⭐⭐
- ⚠️ 解耦性：⭐⭐⭐（依赖中间件顺序）

---

### ADR-4: 字段类型选择

**决策：** 使用 `int` + 输入验证

**备选方案：**
- 使用 `int64`：避免溢出但占用更多内存

**理由：**
1. 与现有代码保持一致（如 `RemainQuota int`）
2. 通过输入验证（1-10000）确保不会溢出
3. 实际业务场景中，10000 次/分钟已经是非常高的限制
4. 节省内存，每个 Token 节省 12 字节

**输入验证规则：**
```go
const (
    MaxRateLimitCount    = 10000  // 最大请求数
    MaxRateLimitDuration = 1440   // 最大周期（24小时）
)
```

**权衡：**
- ✅ 内存占用：⭐⭐⭐⭐⭐
- ✅ 实用性：⭐⭐⭐⭐⭐
- ✅ 安全性：⭐⭐⭐⭐（通过验证保证）

---

### ADR-5: 优先级实现方式

**决策：** 使用 if-else + 配置有效性检查

**备选方案：**
- 策略模式：符合设计模式但过度设计

**理由：**
1. 代码清晰，优先级一目了然
2. 包含防御性检查（配置有效性验证）
3. 不过度设计，符合项目风格
4. 易于维护和调试

**实现示例：**
```go
var totalCount, successCount, duration int
var limitKey string

// 1. 尝试令牌配置
if tokenRateLimitEnabled && isValidConfig(...) {
    totalCount = tokenTotalCount
    successCount = tokenSuccessCount
    duration = tokenDuration
    limitKey = fmt.Sprintf("rateLimit:token:%d", tokenId)
} else {
    // 2. 回退到分组/全局配置
    if groupConfig != nil {
        totalCount = groupConfig.TotalCount
        successCount = groupConfig.SuccessCount
    } else {
        totalCount = globalConfig.TotalCount
        successCount = globalConfig.SuccessCount
    }
    duration = globalDuration
    limitKey = fmt.Sprintf("rateLimit:user:%d", userId)
}
```

**权衡：**
- ✅ 清晰性：⭐⭐⭐⭐⭐
- ✅ 防御性：⭐⭐⭐⭐⭐
- ✅ 可维护性：⭐⭐⭐⭐⭐

## Implementation Plan

### Tasks

**Task 1: 修改 Token 模型**
- 文件：`model/token.go`
- 操作：在 Token struct 中新增 4 个字段
  ```go
  RateLimitEnabled      bool   `json:"rate_limit_enabled" gorm:"default:false"`
  RateLimitTotalCount   int    `json:"rate_limit_total_count" gorm:"default:0;check:rate_limit_total_count >= 0 AND rate_limit_total_count <= 10000"`
  RateLimitSuccessCount int    `json:"rate_limit_success_count" gorm:"default:0;check:rate_limit_success_count >= 0 AND rate_limit_success_count <= rate_limit_total_count"`
  RateLimitDuration     int    `json:"rate_limit_duration" gorm:"default:0;check:rate_limit_duration >= 0 AND rate_limit_duration <= 1440"`
  ```
- 操作：在 `Update()` 方法的 Select 中添加这 4 个字段
- 操作：添加 `BeforeSave` hook 进行验证（修复F4）
  ```go
  func (token *Token) BeforeSave(tx *gorm.DB) error {
      // 无论是否启用，都要验证字段有效性
      if token.RateLimitTotalCount < 0 || token.RateLimitTotalCount > 10000 {
          return errors.New("rate_limit_total_count must be between 0 and 10000")
      }
      if token.RateLimitSuccessCount < 0 || token.RateLimitSuccessCount > token.RateLimitTotalCount {
          return errors.New("rate_limit_success_count must be between 0 and rate_limit_total_count")
      }
      if token.RateLimitDuration < 0 || token.RateLimitDuration > 1440 {
          return errors.New("rate_limit_duration must be between 0 and 1440")
      }
      return nil
  }
  ```
- 操作：在 `Update()` 方法中检测状态变化并清理 Redis Key（修复F3）
  ```go
  func (token *Token) Update() (err error) {
      // 检测是否从启用变为禁用
      oldToken, _ := GetTokenById(token.Id)
      shouldCleanRedis := oldToken != nil && oldToken.RateLimitEnabled && !token.RateLimitEnabled

      defer func() {
          if shouldUpdateRedis(true, err) {
              gopool.Go(func() {
                  err := cacheSetToken(*token)
                  if err != nil {
                      common.SysLog("failed to update token cache: " + err.Error())
                  }
                  // 如果从启用变为禁用，清理 Redis Key
                  if shouldCleanRedis && common.RedisEnabled {
                      rdb := common.RDB
                      ctx := context.Background()
                      rdb.Del(ctx, fmt.Sprintf("rateLimit:token:%d", token.Id))
                      rdb.Del(ctx, fmt.Sprintf("rateLimit:token:%d:success", token.Id))
                  }
              })
          }
      }()
      err = DB.Model(token).Select("name", "status", "expired_time", "remain_quota", "unlimited_quota",
          "model_limits_enabled", "model_limits", "allow_ips", "group", "cross_group_retry",
          "rate_limit_enabled", "rate_limit_total_count", "rate_limit_success_count", "rate_limit_duration").Updates(token).Error
      return err
  }
  ```

**Task 2: 新增 Context Key 常量**
- 文件：`constant/context_key.go`
- 操作：新增 4 个常量
  ```go
  ContextKeyTokenRateLimitEnabled      = "token_rate_limit_enabled"
  ContextKeyTokenRateLimitTotalCount   = "token_rate_limit_total_count"
  ContextKeyTokenRateLimitSuccessCount = "token_rate_limit_success_count"
  ContextKeyTokenRateLimitDuration     = "token_rate_limit_duration"
  ```

**Task 3: 修改认证中间件**
- 文件：`middleware/auth.go`
- 操作：在 Token 验证通过后，将令牌的速率限制配置写入 Context
  ```go
  c.Set(constant.ContextKeyTokenRateLimitEnabled, token.RateLimitEnabled)
  c.Set(constant.ContextKeyTokenRateLimitTotalCount, token.RateLimitTotalCount)
  c.Set(constant.ContextKeyTokenRateLimitSuccessCount, token.RateLimitSuccessCount)
  c.Set(constant.ContextKeyTokenRateLimitDuration, token.RateLimitDuration)
  ```

**Task 4: 修改速率限制中间件**
- 文件：`middleware/model-rate-limit.go`
- 操作：修改 `ModelRequestRateLimit()` 函数
  - 优先检查令牌是否启用独立限流
  - 如果启用，使用令牌配置和 `rateLimit:token:{tokenId}` Key
  - 如果未启用，回退到现有的分组/全局配置和 `rateLimit:user:{userId}` Key
  - 同时支持 Redis 和内存两种实现
- 操作：添加强制检查，防止中间件顺序错误（修复F2）
  ```go
  // 获取令牌配置
  tokenRateLimitEnabled := common.GetContextKeyBool(c, constant.ContextKeyTokenRateLimitEnabled)

  if tokenRateLimitEnabled {
      // 强制检查：如果启用了令牌限流，Context中必须有完整配置
      totalCount := common.GetContextKeyInt(c, constant.ContextKeyTokenRateLimitTotalCount)
      successCount := common.GetContextKeyInt(c, constant.ContextKeyTokenRateLimitSuccessCount)
      duration := common.GetContextKeyInt(c, constant.ContextKeyTokenRateLimitDuration)
      tokenId := c.GetInt("token_id")

      // 如果配置无效，拒绝请求而不是静默回退（修复F9）
      if totalCount <= 0 || successCount <= 0 || duration <= 0 || tokenId <= 0 {
          common.SysLog(fmt.Sprintf("Invalid token rate limit config: tokenId=%d, total=%d, success=%d, duration=%d",
              tokenId, totalCount, successCount, duration))
          abortWithOpenAiMessage(c, http.StatusInternalServerError, "令牌限流配置异常，请联系管理员")
          return
      }

      // 使用令牌级别限流
      // ... 限流逻辑
  }
  ```
- 操作：使用 Redis Lua 脚本确保原子性（修复F1）
  ```go
  // Redis 限流使用 Lua 脚本确保原子性
  luaScript := `
  local key = KEYS[1]
  local limit = tonumber(ARGV[1])
  local ttl = tonumber(ARGV[2])
  local current = redis.call('GET', key)

  if current == false then
      redis.call('SET', key, 1, 'EX', ttl)
      return 1
  end

  current = tonumber(current)
  if current < limit then
      redis.call('INCR', key)
      return current + 1
  else
      return -1
  end
  `

  result, err := rdb.Eval(ctx, luaScript, []string{limitKey}, maxCount, duration).Result()
  if err != nil {
      common.SysLog("Rate limit check failed: " + err.Error())
      abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
      return
  }

  if result.(int64) == -1 {
      abortWithOpenAiMessage(c, http.StatusTooManyRequests,
          fmt.Sprintf("您已达到请求数限制：%d分钟内最多请求%d次", duration/60, maxCount))
      return
  }
  ```
- 操作：为所有限流 Key 设置 TTL（修复F3）
  ```go
  // 设置 TTL 为 duration * 2，确保过期自动清理
  ttl := time.Duration(duration * 2) * time.Second
  rdb.Expire(ctx, limitKey, ttl)
  rdb.Expire(ctx, successKey, ttl)
  ```
- 操作：添加 Redis Key 安全检查
  ```go
  // 确保 tokenId 有效，防止 Redis Key 注入
  tokenId := c.GetInt("token_id")
  if tokenId <= 0 {
      common.SysLog("Invalid tokenId for rate limit")
      abortWithOpenAiMessage(c, http.StatusInternalServerError, "Invalid token")
      return
  }

  // 使用类型安全的方式生成 Key
  limitKey := fmt.Sprintf("rateLimit:token:%d", tokenId)  // %d 确保是整数
  successKey := fmt.Sprintf("rateLimit:token:%d:success", tokenId)
  ```

**Task 5: 修改 Token Controller**
- 文件：`controller/token.go`
- 操作：修改 `AddToken()` 函数，在 cleanToken 中添加 4 个新字段
- 操作：修改 `UpdateToken()` 函数，在更新逻辑中添加 4 个新字段
- 操作：添加权限检查（修复F8 - 增强权限控制）
  ```go
  // 确保用户只能修改自己的令牌
  token, err := model.GetTokenByIds(tokenId, userId)
  if err != nil {
      c.JSON(http.StatusOK, gin.H{
          "success": false,
          "message": "令牌不存在或无权访问",
      })
      return
  }

  // 检查用户角色限制（修复F8）
  user, _ := model.GetUserById(userId, false)
  if user != nil && user.Role == common.RoleCommonUser {
      // 普通用户限制最大配额
      if token.RateLimitTotalCount > 1000 {
          c.JSON(http.StatusOK, gin.H{
              "success": false,
              "message": "普通用户的令牌限流配置不能超过 1000 次/分钟",
          })
          return
      }
  }
  ```
- 操作：添加输入验证逻辑（修复F4 - 无论是否启用都验证）
  ```go
  // 验证速率限制配置 - 无论是否启用都要验证
  if token.RateLimitTotalCount < 0 || token.RateLimitTotalCount > 10000 {
      c.JSON(http.StatusOK, gin.H{
          "success": false,
          "message": "总请求数必须在 0-10000 之间",
      })
      return
  }
  if token.RateLimitSuccessCount < 0 || token.RateLimitSuccessCount > token.RateLimitTotalCount {
      c.JSON(http.StatusOK, gin.H{
          "success": false,
          "message": "成功请求数必须在 0 到总请求数之间",
      })
      return
  }
  if token.RateLimitDuration < 0 || token.RateLimitDuration > 1440 {
      c.JSON(http.StatusOK, gin.H{
          "success": false,
          "message": "限流周期必须在 0-1440 分钟之间",
      })
      return
  }

  // 如果启用了限流，确保配置值有效
  if token.RateLimitEnabled {
      if token.RateLimitTotalCount == 0 || token.RateLimitSuccessCount == 0 || token.RateLimitDuration == 0 {
          c.JSON(http.StatusOK, gin.H{
              "success": false,
              "message": "启用令牌限流时，所有配置值必须大于 0",
          })
          return
      }
  }
  ```
- 操作：添加配置合理性警告
  ```go
  // 配置合理性检查（记录警告但不阻止）
  if token.RateLimitTotalCount > 1000 {
      common.SysLog(fmt.Sprintf("High rate limit configured: token_id=%d, count=%d",
          token.Id, token.RateLimitTotalCount))
  }
  ```
- 操作：确保 `GetToken()` 和 `GetAllTokens()` 返回新字段（GORM 自动处理）

**Task 6: 修改 Token Delete 方法**
- 文件：`model/token.go`
- 操作：在 `Delete()` 方法中添加 Redis Key 清理逻辑（修复F3 - 完善清理机制）
  ```go
  func (token *Token) Delete() (err error) {
      defer func() {
          if shouldUpdateRedis(true, err) {
              gopool.Go(func() {
                  err := cacheDeleteToken(token.Key)
                  if err != nil {
                      common.SysLog("failed to delete token cache: " + err.Error())
                  }

                  // 删除令牌时清理限流计数器
                  if common.RedisEnabled {
                      rdb := common.RDB
                      ctx := context.Background()
                      // 清理令牌级别的限流 Key
                      rdb.Del(ctx, fmt.Sprintf("rateLimit:token:%d", token.Id))
                      rdb.Del(ctx, fmt.Sprintf("rateLimit:token:%d:success", token.Id))
                      common.SysLog(fmt.Sprintf("Cleaned rate limit keys for token %d", token.Id))
                  }
              })
          }
      }()
      err = DB.Delete(token).Error
      return err
  }
  ```
- 操作：添加定期清理孤儿 Key 的任务（修复F3）
  ```go
  // 在 main.go 或专门的清理任务中添加
  func CleanOrphanRateLimitKeys() {
      if !common.RedisEnabled {
          return
      }

      rdb := common.RDB
      ctx := context.Background()

      // 扫描所有 rateLimit:token:* 的 Key
      iter := rdb.Scan(ctx, 0, "rateLimit:token:*", 100).Iterator()
      for iter.Next(ctx) {
          key := iter.Val()
          // 提取 tokenId
          parts := strings.Split(key, ":")
          if len(parts) >= 3 {
              tokenId, _ := strconv.Atoi(parts[2])
              // 检查 token 是否存在
              _, err := GetTokenById(tokenId)
              if err != nil {
                  // Token 不存在，删除 Key
                  rdb.Del(ctx, key)
                  common.SysLog(fmt.Sprintf("Cleaned orphan rate limit key: %s", key))
              }
          }
      }
  }

  // 建议每小时运行一次
  // go func() {
  //     ticker := time.NewTicker(1 * time.Hour)
  //     for range ticker.C {
  //         CleanOrphanRateLimitKeys()
  //     }
  // }()
  ```

**Task 7: 修改前端令牌编辑页面**
- 文件：`web/src/components/table/tokens/modals/EditTokenModal.jsx`
- 操作：在 `getInitValues()` 中添加 4 个新字段的初始值
  ```jsx
  rate_limit_enabled: false,
  rate_limit_total_count: 60,
  rate_limit_success_count: 60,
  rate_limit_duration: 1,
  ```
- 操作：在表单中添加速率限制配置区域（新增第四个 Card）
  - Switch：是否启用令牌级别限流
  - InputNumber：总请求数限制（验证规则：0-10000）
  - InputNumber：成功请求数限制（验证规则：0-总请求数）（修复F7）
  - InputNumber：限流周期（分钟，验证规则：0-1440）
  - 快捷设置按钮：1分钟(60次)、5分钟(300次)、10分钟(600次)
- 操作：只有启用时才显示配置字段（使用条件渲染）
- 操作：添加前端验证规则（修复F7 - 完善验证）
  ```jsx
  <Form.InputNumber
      field="rate_limit_total_count"
      label={t('总请求数限制')}
      disabled={!values.rate_limit_enabled}
      rules={[
          { required: values.rate_limit_enabled, message: t('请输入总请求数') },
          { type: 'number', min: 0, max: 10000, message: t('必须在 0-10000 之间') }
      ]}
      extraText={t('每个周期内允许的最大请求总数（包括成功和失败）')}
  />
  <Form.InputNumber
      field="rate_limit_success_count"
      label={t('成功请求数限制')}
      disabled={!values.rate_limit_enabled}
      rules={[
          { required: values.rate_limit_enabled, message: t('请输入成功请求数') },
          { type: 'number', min: 0, message: t('必须大于等于 0') },
          {
              validator: (rule, value) => {
                  const totalCount = formApiRef.current?.getValue('rate_limit_total_count') || 0;
                  if (value > totalCount) {
                      return Promise.reject(t('成功请求数不能超过总请求数'));
                  }
                  return Promise.resolve();
              }
          }
      ]}
      extraText={t('每个周期内允许的最大成功请求数')}
  />
  <Form.InputNumber
      field="rate_limit_duration"
      label={t('限流周期（分钟）')}
      disabled={!values.rate_limit_enabled}
      rules={[
          { required: values.rate_limit_enabled, message: t('请输入限流周期') },
          { type: 'number', min: 0, max: 1440, message: t('必须在 0-1440 分钟之间') }
      ]}
      extraText={t('限流的时间窗口，单位为分钟（1440分钟=24小时）')}
  />
  ```
- 操作：添加快捷设置按钮（修复F16 - 明确行为）
  ```jsx
  <Form.Slot label={t('快捷设置')}>
      <Space wrap>
          <Button
              theme='light'
              type='tertiary'
              disabled={!values.rate_limit_enabled}
              onClick={() => {
                  formApiRef.current?.setValues({
                      rate_limit_duration: 1,
                      rate_limit_total_count: 60,
                      rate_limit_success_count: 60
                  });
              }}
          >
              {t('1分钟/60次')}
          </Button>
          <Button
              theme='light'
              type='tertiary'
              disabled={!values.rate_limit_enabled}
              onClick={() => {
                  formApiRef.current?.setValues({
                      rate_limit_duration: 5,
                      rate_limit_total_count: 300,
                      rate_limit_success_count: 300
                  });
              }}
          >
              {t('5分钟/300次')}
          </Button>
          <Button
              theme='light'
              type='tertiary'
              disabled={!values.rate_limit_enabled}
              onClick={() => {
                  formApiRef.current?.setValues({
                      rate_limit_duration: 10,
                      rate_limit_total_count: 600,
                      rate_limit_success_count: 600
                  });
              }}
          >
              {t('10分钟/600次')}
          </Button>
      </Space>
  </Form.Slot>
  ```
- 操作：添加国际化翻译（修复F12）
  - 在 `web/src/i18n/zh.json` 和 `web/src/i18n/en.json` 中添加翻译
  - 翻译键：`rate_limit_enabled`, `rate_limit_total_count`, `rate_limit_success_count`, `rate_limit_duration`

### Acceptance Criteria

**AC1: Token 模型支持新字段**
- Given: Token 模型已定义
- When: 创建或查询 Token
- Then: 可以正确读写 4 个新字段

**AC2: 令牌级别限流生效**
- Given: 令牌启用独立限流，配置为 10 次/分钟
- When: 使用该令牌在 1 分钟内请求 11 次
- Then: 第 11 次请求返回 429 Too Many Requests

**AC3: 令牌独立计数**
- Given: 用户有 2 个令牌，令牌 A 启用独立限流（10次/分钟），令牌 B 未启用
- When: 令牌 A 请求 10 次，令牌 B 请求 10 次
- Then: 令牌 A 的第 11 次请求被拒绝，令牌 B 不受影响

**AC4: 向后兼容**
- Given: 现有令牌未启用独立限流（RateLimitEnabled = false）
- When: 使用该令牌请求
- Then: 继续使用分组/全局配置，行为与改造前一致

**AC5: 区分总请求数和成功请求数**
- Given: 令牌配置总请求数 20 次/分钟，成功请求数 10 次/分钟
- When: 在 1 分钟内成功请求 10 次
- Then: 第 11 次成功请求被拒绝，但失败请求仍可继续（直到总数达到 20）

**AC6: 前端表单正确显示和提交**
- Given: 打开令牌编辑页面
- When: 启用令牌级别限流并配置参数
- Then: 表单正确显示配置字段，提交后数据正确保存

**AC7: API 正确返回新字段**
- Given: 令牌已配置速率限制
- When: 调用 GET /api/token/:id
- Then: 返回的 JSON 包含 4 个新字段及其正确的值

**AC8: Redis Key 正确生成**
- Given: 令牌 ID 为 5001，启用独立限流
- When: 使用该令牌请求
- Then: Redis 中生成 Key `rateLimit:token:5001` 和 `rateLimit:token:5001:success`

**AC9: 输入验证生效（修复F4）**
- Given: 尝试创建令牌，配置总请求数为 -1，RateLimitEnabled 为 false
- When: 提交表单
- Then: 后端返回错误 "rate_limit_total_count must be between 0 and 10000"

**AC10: Redis Key 清理**
- Given: 令牌 ID 为 5001，已启用独立限流并产生了计数器
- When: 删除该令牌
- Then: Redis 中的 `rateLimit:token:5001` 和 `rateLimit:token:5001:success` 被清理

**AC11: 配置无效时拒绝请求（修复F2和F9）**
- Given: 令牌启用独立限流，但配置值为 0（数据异常）
- When: 使用该令牌请求
- Then: 系统返回 500 错误 "令牌限流配置异常，请联系管理员"，而不是静默回退

**AC12: 并发请求计数准确（修复F1）**
- Given: 令牌配置 10 次/分钟
- When: 100 个并发请求同时到达
- Then: 前 10 个请求通过，后续请求被拒绝，计数准确无误

**AC13: Redis Key 注入防护**
- Given: 尝试使用异常的 tokenId（如负数或 0）
- When: 发起请求
- Then: 系统记录错误日志并返回 500 错误，不生成 Redis Key

**AC14: 权限控制生效（修复F8）**
- Given: 普通用户尝试设置令牌限流为 5000 次/分钟
- When: 调用 UpdateToken API
- Then: 返回错误 "普通用户的令牌限流配置不能超过 1000 次/分钟"

**AC15: 配置合理性警告**
- Given: 配置总请求数为 5000（超过建议值 1000）
- When: 保存配置
- Then: 配置成功保存，但系统记录警告日志

**AC16: 状态变化时清理 Redis Key（修复F3）**
- Given: 令牌 ID 为 5001，已启用独立限流并产生了计数器
- When: 将 RateLimitEnabled 从 true 改为 false
- Then: Redis 中的 `rateLimit:token:5001` 和 `rateLimit:token:5001:success` 被清理

**AC17: Redis Key 自动过期（修复F3）**
- Given: 令牌配置 10 次/1分钟
- When: 使用该令牌请求后
- Then: Redis Key 的 TTL 被设置为 2 分钟（duration * 2）

**AC18: 前端成功请求数验证（修复F7）**
- Given: 在前端表单中设置总请求数为 100，成功请求数为 200
- When: 提交表单
- Then: 前端显示验证错误 "成功请求数不能超过总请求数"

**AC19: 中间件顺序错误检测（修复F2）**
- Given: 路由配置错误，限流中间件在认证中间件之前执行
- When: 使用启用独立限流的令牌请求
- Then: 系统返回 500 错误，而不是静默回退到用户级别限流

**AC20: BeforeSave hook 验证（修复F4）**
- Given: 尝试通过数据库直接插入异常数据（total_count = -1）
- When: 执行 Insert 或 Update
- Then: GORM BeforeSave hook 拦截并返回错误

## Additional Context

### 对抗性审查修复总结

本规格经过对抗性审查后，已修复以下关键问题：

**🔴 Critical 级别修复（4个）：**

1. **F1 - 并发竞态条件**
   - 问题：高并发场景下计数器原子性问题
   - 修复：在 Task 4 中使用 Redis Lua 脚本确保 INCR + TTL + 比较操作的原子性
   - 验证：AC12 增强为 100 并发请求测试

2. **F2 - 中间件顺序依赖**
   - 问题：路由配置错误会导致静默失败
   - 修复：在 Task 4 中添加强制检查，如果 Context 中没有值则拒绝请求（返回 500）
   - 验证：AC19 测试中间件顺序错误检测

3. **F3 - Redis Key 清理不完整**
   - 问题：多种场景会导致 Key 泄漏
   - 修复：
     - Task 1 中在 Update() 方法检测状态变化时清理
     - Task 4 中为所有 Key 设置 TTL（duration * 2）
     - Task 6 中添加定期清理孤儿 Key 的任务
   - 验证：AC16（状态变化清理）、AC17（TTL 设置）

4. **F4 - 输入验证逻辑漏洞**
   - 问题：只在启用时验证，可以绕过验证插入异常数据
   - 修复：
     - Task 1 中添加 BeforeSave hook，无论是否启用都验证
     - Task 1 中添加数据库 CHECK 约束
     - Task 5 中无论是否启用都验证所有字段
   - 验证：AC9（后端验证）、AC20（BeforeSave hook）

**⚠️ High 级别修复（5个）：**

5. **F5 - 时间窗口边界攻击**
   - 状态：已在 ADR-2 中记录局限性，建议未来实现突发检测

6. **F6 - 缓存一致性**
   - 修复：Task 1 中在 Update() 方法中清理缓存，Task 6 中在 Delete() 方法中清理缓存

7. **F7 - 前端验证不完整**
   - 问题：没有验证 success_count <= total_count
   - 修复：Task 7 中添加自定义验证器，实时验证成功请求数不超过总请求数
   - 验证：AC18 测试前端验证

8. **F8 - 权限控制不够细粒度**
   - 问题：没有考虑角色限制
   - 修复：Task 5 中添加用户角色检查，普通用户限制最大 1000 次/分钟
   - 验证：AC14 增强为角色限制测试

9. **F9 - 防御性检查回退策略**
   - 问题：静默回退会让用户误以为配置生效
   - 修复：Task 4 中改为拒绝请求并返回 500 错误，而不是静默回退
   - 验证：AC11 修改为测试拒绝请求

**⚠️ Medium 级别修复（2个）：**

10. **F11 - TTL 设置缺失**
    - 修复：Task 4 中明确设置 TTL 为 duration * 2
    - 验证：AC17 测试 TTL 设置

11. **F12 - 国际化翻译缺失**
    - 修复：Task 7 中添加翻译文件修改说明

**💡 Low 级别修复（1个）：**

12. **F16 - 快捷按钮设计不明确**
    - 修复：Task 7 中明确快捷按钮的行为和默认值

### Dependencies

- Go 1.18+
- GORM v2
- Redis（可选，也支持内存限流）
- React 18+
- Semi-UI

### Testing Strategy

**单元测试：**
1. Token 模型的 CRUD 操作
2. Token 模型的 BeforeSave hook 验证（F4）
3. 限流中间件的优先级逻辑
4. Redis Key 生成逻辑
5. 输入验证逻辑（边界值测试，包括 0 值）（F4）
6. 防御性检查逻辑（配置无效时拒绝请求）（F9）
7. Redis Key 注入防护测试
8. 权限控制测试（包括角色限制）（F8）
9. Redis Lua 脚本的原子性测试（F1）

**集成测试：**
1. 令牌级别限流的端到端测试
2. 多令牌并发请求测试（100+ 并发）（F1）
3. 向后兼容性测试
4. Redis Key 清理测试（Delete、Update 状态变化）（F3）
5. 配置更新后的缓存同步测试（F6）
6. Redis 和内存两种实现的一致性测试
7. 时间窗口边界测试
8. 中间件顺序错误测试（F2）
9. TTL 自动过期测试（F3）

**安全测试：**
1. Redis Key 注入攻击测试
2. 权限绕过测试（包括角色绕过）（F8）
3. 配置篡改测试（通过数据库直接修改）（F4）
4. 时间窗口边界攻击测试（F5）
5. 并发竞态条件测试（F1）

**手动测试：**
1. 前端表单的交互测试（包括成功请求数验证）（F7）
2. 不同配置组合的限流效果测试
3. Redis 中 Key 的正确性验证（包括 TTL）
4. 边界条件测试（0、负数、超大值）
5. 时间窗口切换时的行为验证
6. 快捷设置按钮的行为测试（F16）
7. 国际化翻译测试（F12）

**压力测试（新增）：**
1. 1000 QPS 并发请求测试，验证计数准确性（F1）
2. 10 万个令牌启用独立限流的内存占用测试
3. Redis Key 泄漏测试（长时间运行）（F3）
4. 定期清理任务的性能影响测试（F3）

### Notes

**实现注意事项：**

1. **中间件修改要小心** - 限流中间件是核心逻辑，修改时要确保不影响现有功能
2. **Context 传值** - 需要在认证中间件中将令牌配置写入 Context，供限流中间件使用
3. **Redis 和内存两种实现** - 修改时要同时支持两种限流实现
4. **前端表单验证** - 确保用户输入的值合法（如不能为负数）
5. **数据库迁移** - 使用 GORM AutoMigrate，确保现有数据不受影响
6. **防御性编程** - 在中间件中添加配置有效性检查，配置无效时**拒绝请求**而不是静默回退（F9）
7. **Redis Key 清理** - 删除令牌时要清理对应的限流计数器，避免内存泄漏（F3）
8. **并发安全** - 使用 Redis Lua 脚本确保并发请求计数准确（F1）
9. **缓存一致性** - 更新令牌配置后要同步更新 Redis 缓存（F6）
10. **输入验证** - 无论是否启用限流，都要验证所有字段的有效性（F4）
11. **TTL 设置** - 为所有限流 Key 设置 TTL（duration * 2），确保自动过期（F3）
12. **中间件顺序** - 确保路由配置中认证中间件在限流中间件之前，添加强制检查（F2）
13. **角色权限** - 根据用户角色限制可配置的最大值（F8）
14. **BeforeSave Hook** - 在 GORM 模型中添加 BeforeSave hook 进行数据验证（F4）
15. **定期清理** - 实现定期清理孤儿 Key 的任务，建议每小时运行一次（F3）

**性能考虑：**

1. **Redis Key 数量增加** - 启用令牌级别限流后，每个令牌会有独立的 Redis Key
2. **建议** - 只对需要独立限流的令牌启用该功能，大部分令牌继续使用分组/全局配置
3. **TTL 设置** - 为限流计数器设置合理的 TTL（duration * 2），避免过期 Key 占用内存（F3）
4. **批量清理** - 定期清理已删除令牌的残留 Key，避免内存泄漏（F3）
5. **Lua 脚本性能** - Redis Lua 脚本执行是原子的，但会阻塞其他命令，需要保持脚本简洁（F1）
6. **并发测试** - 在生产环境部署前，进行 1000+ QPS 的压力测试（F1）

**向后兼容性：**

1. **默认值** - 所有新字段默认值确保现有令牌行为不变
2. **渐进式迁移** - 可以逐步为需要的令牌启用独立限流
3. **无需一次性改造** - 现有系统可以平滑升级
4. **数据库约束** - 添加 CHECK 约束确保数据完整性，但不影响现有数据（F4）

**故障模式预防：**

1. **输入验证** - 前后端双重验证 + BeforeSave hook + 数据库约束，四重防护（F4）
2. **配置无效处理** - 中间件中检查配置有效性，异常时拒绝请求并返回明确错误（F9）
3. **日志记录** - 记录配置异常、清理操作、高配额警告，便于排查问题
4. **缓存更新** - 确保数据库和 Redis 缓存的一致性（F6）
5. **资源清理** - 删除令牌时清理相关资源，状态变化时清理 Key，定期清理孤儿 Key（F3）
6. **中间件顺序检查** - 在限流中间件中强制检查 Context 值，防止配置错误（F2）
7. **原子操作** - 使用 Redis Lua 脚本确保并发安全（F1）
8. **角色限制** - 根据用户角色限制配置范围，防止滥用（F8）

**部署建议：**

1. **灰度发布** - 先在少量令牌上启用独立限流，观察效果
2. **监控指标** - 监控 Redis 内存使用、Key 数量、限流触发频率
3. **回滚方案** - 准备手动回滚 SQL 脚本（删除新增字段）
4. **文档更新** - 更新用户文档，说明新功能和使用方法
5. **定期清理任务** - 部署后启动定期清理任务，监控清理效果（F3）

---

## 安全考虑

### 🔴 高优先级安全措施（必须实现）

**1. Redis Key 注入防护**
```go
// 确保 tokenId 有效，防止 Redis Key 注入
tokenId := c.GetInt("token_id")
if tokenId <= 0 {
    common.SysLog("Invalid tokenId for rate limit")
    return error
}

// 使用类型安全的方式生成 Key（%d 确保是整数）
limitKey := fmt.Sprintf("rateLimit:token:%d", tokenId)
```

**威胁：** 如果 tokenId 来自不可信输入，攻击者可能注入特殊字符影响 Redis 操作
**防御：** 使用 `%d` 格式化确保类型安全，验证 tokenId 有效性

---

**2. 权限控制**
```go
// 确保用户只能修改自己的令牌
token, err := model.GetTokenByIds(tokenId, userId)
if err != nil {
    return errors.New("令牌不存在或无权访问")
}
```

**威胁：** 攻击者可能尝试修改其他用户的令牌配置
**防御：** 在所有修改操作中验证令牌所有权

---

**3. 输入验证**
```go
// 前后端双重验证
if token.RateLimitTotalCount <= 0 || token.RateLimitTotalCount > 10000 {
    return errors.New("总请求数必须在 1-10000 之间")
}
```

**威胁：** 异常配置值可能导致系统崩溃或绕过限流
**防御：** 前后端双重验证，确保配置在合理范围内

---

### ⚠️ 中优先级安全措施（建议实现）

**4. 配置合理性检查**
```go
// 警告过高的配置
if token.RateLimitTotalCount > 1000 {
    logger.Warn("High rate limit configured", map[string]interface{}{
        "token_id": tokenId,
        "total_count": token.RateLimitTotalCount,
        "recommendation": "建议不超过 1000 次/分钟",
    })
}
```

**威胁：** 过高的限流配置可能被滥用
**防御：** 记录警告日志，便于管理员发现异常配置

---

**5. 时间窗口边界问题**

**威胁：** 固定时间窗口算法存在边界问题，攻击者可能在窗口切换时发起双倍请求
**防御：**
- 在文档中明确说明局限性（已在 ADR-2 中记录）
- 对于高安全要求场景，建议使用滑动窗口算法
- 添加额外的突发流量检测机制

---

### 💡 低优先级安全措施（未来考虑）

**6. 审计日志**
- 记录所有配置变更（用户、时间、旧值、新值）
- 记录限流触发事件（IP、时间、原因）
- 记录异常回退行为

**7. 监控和告警**
- 添加 Prometheus 指标：`rate_limit_rejected_total{token_id, reason}`
- 配置告警规则：限流触发频率过高时告警
- 创建监控仪表板

**8. 数据保护**
- Redis 连接使用 TLS
- 敏感日志脱敏处理
- 定期清理过期的限流计数器

---

### 安全威胁总结

| 威胁 | 等级 | 防御措施 | 状态 |
|------|------|---------|------|
| Redis Key 注入 | 🔴 高 | 类型安全的 Key 生成 + tokenId 验证 | ✅ 已实现 |
| 权限绕过 | 🔴 高 | 令牌所有权验证 | ✅ 已实现 |
| 配置篡改 | ⚠️ 中 | 输入验证 + 合理性检查 | ✅ 已实现 |
| 时间窗口边界攻击 | ⚠️ 中 | 文档说明 + 算法升级建议 | ✅ 已记录 |
| 令牌枚举 | ⚠️ 中 | 统一错误消息 + IP 限流 | 💡 未来考虑 |
| 审计缺失 | 💡 低 | 审计日志系统 | 💡 未来考虑 |
| 监控盲区 | 💡 低 | 监控和告警系统 | 💡 未来考虑 |

---

## 设计哲学与第一性原理

### 核心问题的本质

**速率限制的本质：**
```
速率限制 = 在给定时间窗口内，限制某个实体的操作次数

核心要素：
1. 实体（Who）- 用户 or 令牌
2. 操作（What）- API 请求
3. 时间窗口（When）- 可配置的分钟数
4. 次数限制（How many）- 总请求数 + 成功请求数
```

**为什么需要令牌级别限流？**

本质是**资源隔离**，而不仅仅是更细粒度的限流。

类比：就像 Docker 容器隔离资源，每个令牌应该有独立的资源配额，避免低优先级操作影响高优先级操作。

---

### 关键设计权衡

**1. 双重限制的必要性**

当前设计：总请求数 + 成功请求数

**为什么不简化为单一限制？**
- 单一限制更简单，但无法防止恶意用户通过故意发送错误请求来探测系统
- 双重限制虽然增加复杂度，但提供了更好的保护

**未来优化方向：**
- 考虑简化为：总请求数 + 错误率监控
- 错误率监控可以发现异常行为，同时简化配置

---

**2. 固定窗口算法的选择**

**三种算法的本质权衡：**

| 算法 | 时间复杂度 | 空间复杂度 | 精确度 | 适用场景 |
|------|-----------|-----------|--------|---------|
| 固定窗口 | O(1) | O(1) | ⭐⭐⭐ | 高并发，可接受边界问题 |
| 滑动窗口 | O(N) | O(N) | ⭐⭐⭐⭐⭐ | 精确限流，性能要求不高 |
| 令牌桶 | O(1) | O(1) | ⭐⭐⭐⭐ | 允许突发流量 |

**当前选择固定窗口的理由：**
- 性能是首要考虑（高并发场景）
- 边界问题的影响有限（最多 2N 次请求，而不是无限次）
- 实现简单，易于维护

**未来演进方向：**
- 作为可选的限流算法，通过配置切换
- 对于高安全要求场景，提供滑动窗口或令牌桶选项

---

**3. 配置优先级的特殊性原则**

```
令牌配置 > 分组配置 > 全局配置

符合特殊性原则：越具体的配置优先级越高
```

**配置回退的哲学问题：**

当前实现：如果令牌配置无效（值为 0），回退到分组/全局配置

**两种观点：**
1. **防御性：** 避免系统崩溃，提供降级方案
2. **严格性：** 尊重用户选择，即使配置为 0 也应该拒绝所有请求

**当前选择防御性的理由：**
- 避免配置错误导致服务不可用
- 提供更好的用户体验

**潜在问题：**
- 可能隐藏配置错误，用户不知道配置无效
- 建议：记录警告日志，便于发现问题

---

**4. 数据存储的权衡**

**配置数据（Token 模型）：**
- 存储：数据库（持久化）
- 缓存：Redis（性能）
- 权衡：强一致性 + 高可用性

**限流计数器：**
- 存储：Redis（临时数据）
- 备选：内存（无 Redis 时）
- 权衡：性能优先，允许丢失（重启后重置）

**Redis 重启的风险：**
- 所有限流计数器丢失，用户可以立即发起新请求
- 攻击者可能利用这个窗口绕过限流
- **接受的风险：** Redis 重启是罕见事件，影响有限

---

### 设计原则总结

**1. 性能优先**
- 选择固定窗口算法（O(1) 时间和空间复杂度）
- 通过 Context 传值避免重复查询
- 使用 Redis 缓存提升性能

**2. 安全第二**
- Redis Key 注入防护
- 权限控制
- 输入验证

**3. 可维护性第三**
- 不过度设计（避免策略模式等复杂模式）
- 清晰的代码结构
- 完善的文档和注释

**4. 向后兼容**
- 默认值确保现有系统行为不变
- 渐进式迁移
- 无需一次性改造

---

### 未来演进方向

**短期（当前版本）：**
- ✅ 实现令牌级别独立限流
- ✅ 输入验证和防御性编程
- ✅ Redis Key 清理机制

**中期（下一版本）：**
- 💡 添加基础审计功能（记录配置变更和限流触发）
- 💡 更严格的配置验证（不回退，而是拒绝请求）
- 💡 配置合理性建议（前端提示）

**长期（未来版本）：**
- 💡 支持多种限流算法（滑动窗口、令牌桶）
- 💡 简化为单一限制 + 错误率监控
- 💡 完整的监控和告警系统
- 💡 基于角色的访问控制（RBAC）

---

### 关键洞察

**1. 令牌级别限流的本质是资源隔离**
- 不仅仅是更细粒度的限流
- 而是为不同用途的令牌提供独立的资源配额

**2. 固定窗口算法是当前场景的最佳选择**
- 性能 > 精确度
- 边界问题的影响有限
- 未来可以通过配置切换到其他算法

**3. 防御性编程 vs 严格验证**
- 当前选择防御性（配置无效时回退）
- 未来可以考虑更严格的验证（配置无效时拒绝）

**4. 审计功能的重要性**
- 当前缺少审计功能是一个重要的安全缺陷
- 应该从"未来考虑"提升到"建议实现"

**5. 双重限制的必要性**
- 总请求数 + 成功请求数提供了更好的保护
- 未来可以考虑简化为：总请求数 + 错误率监控
