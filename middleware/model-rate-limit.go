package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	ModelRequestRateLimitCountMark        = "MRRL"
	ModelRequestRateLimitSuccessCountMark = "MRRLS"
)

// 检查Redis中的请求限制
func checkRedisRateLimit(ctx context.Context, rdb *redis.Client, key string, maxCount int, duration int64) (bool, error) {
	// 如果maxCount为0，表示不限制
	if maxCount == 0 {
		return true, nil
	}

	// 获取当前计数
	length, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		return false, err
	}

	// 如果未达到限制，允许请求
	if length < int64(maxCount) {
		return true, nil
	}

	// 检查时间窗口
	oldTimeStr, _ := rdb.LIndex(ctx, key, -1).Result()
	oldTime, err := time.Parse(timeFormat, oldTimeStr)
	if err != nil {
		return false, err
	}

	nowTimeStr := time.Now().Format(timeFormat)
	nowTime, err := time.Parse(timeFormat, nowTimeStr)
	if err != nil {
		return false, err
	}
	// 如果在时间窗口内已达到限制，拒绝请求
	subTime := nowTime.Sub(oldTime).Seconds()
	if int64(subTime) < duration {
		rdb.Expire(ctx, key, time.Duration(setting.ModelRequestRateLimitDurationMinutes)*time.Minute)
		return false, nil
	}

	return true, nil
}

// 记录Redis请求
func recordRedisRequest(ctx context.Context, rdb *redis.Client, key string, maxCount int) {
	// 如果maxCount为0，不记录请求
	if maxCount == 0 {
		return
	}

	now := time.Now().Format(timeFormat)
	rdb.LPush(ctx, key, now)
	rdb.LTrim(ctx, key, 0, int64(maxCount-1))
	rdb.Expire(ctx, key, time.Duration(setting.ModelRequestRateLimitDurationMinutes)*time.Minute)
}

// Redis限流处理器 - 使用Lua脚本确保原子性
func redisRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, limitKey, successKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		rdb := common.RDB

		// Lua脚本：原子性地检查和增加计数器
		luaScript := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])

		if limit == 0 then
			return 1
		end

		local current = redis.call('GET', key)

		if current == false then
			redis.call('SET', key, 1, 'EX', ttl)
			return 1
		end

		current = tonumber(current)
		if current < limit then
			redis.call('INCR', key)
			redis.call('EXPIRE', key, ttl)
			return current + 1
		else
			return -1
		end
		`

		// 1. 检查总请求数限制
		ttl := duration * 2 // TTL 设置为 duration 的 2 倍
		result, err := rdb.Eval(ctx, luaScript, []string{limitKey}, totalMaxCount, ttl).Result()
		if err != nil {
			common.SysLog("Rate limit check failed: " + err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}

		if result.(int64) == -1 {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests,
				fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次", duration/60, totalMaxCount))
			return
		}

		// 2. 处理请求
		c.Next()

		// 3. 如果请求成功，检查并记录成功请求数
		if c.Writer.Status() < 400 {
			result, err := rdb.Eval(ctx, luaScript, []string{successKey}, successMaxCount, ttl).Result()
			if err != nil {
				common.SysLog("Success rate limit check failed: " + err.Error())
				return
			}

			if result.(int64) == -1 {
				// 成功请求数已达上限，但请求已经处理完成，只记录日志
				common.SysLog(fmt.Sprintf("Success rate limit exceeded for key: %s", successKey))
			}
		}
	}
}

// 内存限流处理器
func memoryRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, limitKey, successKey string) gin.HandlerFunc {
	inMemoryRateLimiter.Init(time.Duration(duration) * time.Second)

	return func(c *gin.Context) {
		// 1. 检查总请求数限制（当totalMaxCount为0时跳过）
		if totalMaxCount > 0 && !inMemoryRateLimiter.Request(limitKey, totalMaxCount, duration) {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests,
				fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次", duration/60, totalMaxCount))
			return
		}

		// 2. 检查成功请求数限制
		// 使用一个临时key来检查限制，这样可以避免实际记录
		checkKey := successKey + "_check"
		if !inMemoryRateLimiter.Request(checkKey, successMaxCount, duration) {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests,
				fmt.Sprintf("您已达到成功请求数限制：%d分钟内最多成功请求%d次", duration/60, successMaxCount))
			return
		}

		// 3. 处理请求
		c.Next()

		// 4. 如果请求成功，记录到实际的成功请求计数中
		if c.Writer.Status() < 400 {
			inMemoryRateLimiter.Request(successKey, successMaxCount, duration)
		}
	}
}

// ModelRequestRateLimit 模型请求限流中间件
func ModelRequestRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 在每个请求时检查是否启用限流
		if !setting.ModelRequestRateLimitEnabled {
			c.Next()
			return
		}

		var totalMaxCount, successMaxCount int
		var duration int64
		var limitKey, successKey string

		// 获取令牌级别的速率限制配置
		tokenRateLimitEnabled := common.GetContextKeyBool(c, constant.ContextKeyTokenRateLimitEnabled)

		if tokenRateLimitEnabled {
			// 优先级1: 使用令牌级别配置
			totalMaxCount = common.GetContextKeyInt(c, constant.ContextKeyTokenRateLimitTotalCount)
			successMaxCount = common.GetContextKeyInt(c, constant.ContextKeyTokenRateLimitSuccessCount)
			durationMinutes := common.GetContextKeyInt(c, constant.ContextKeyTokenRateLimitDuration)
			tokenId := c.GetInt("token_id")

			// 强制检查：如果启用了令牌限流，Context中必须有完整配置
			if totalMaxCount <= 0 || successMaxCount <= 0 || durationMinutes <= 0 || tokenId <= 0 {
				common.SysLog(fmt.Sprintf("Invalid token rate limit config: tokenId=%d, total=%d, success=%d, duration=%d",
					tokenId, totalMaxCount, successMaxCount, durationMinutes))
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "令牌限流配置异常，请联系管理员")
				return
			}

			duration = int64(durationMinutes * 60)
			// 使用类型安全的方式生成 Key
			limitKey = fmt.Sprintf("rateLimit:token:%d", tokenId)
			successKey = fmt.Sprintf("rateLimit:token:%d:success", tokenId)

			// 记录高配额警告
			if totalMaxCount > 1000 {
				common.SysLog(fmt.Sprintf("High rate limit configured: token_id=%d, count=%d", tokenId, totalMaxCount))
			}
		} else {
			// 优先级2: 使用分组配置或全局配置
			duration = int64(setting.ModelRequestRateLimitDurationMinutes * 60)
			totalMaxCount = setting.ModelRequestRateLimitCount
			successMaxCount = setting.ModelRequestRateLimitSuccessCount

			// 获取分组
			group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
			if group == "" {
				group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
			}

			// 获取分组的限流配置
			groupTotalCount, groupSuccessCount, found := setting.GetGroupRateLimit(group)
			if found {
				totalMaxCount = groupTotalCount
				successMaxCount = groupSuccessCount
			}

			// 使用用户级别的 Key
			userId := strconv.Itoa(c.GetInt("id"))
			limitKey = fmt.Sprintf("rateLimit:user:%s", userId)
			successKey = fmt.Sprintf("rateLimit:user:%s:success", userId)
		}

		// 根据存储类型选择并执行限流处理器
		if common.RedisEnabled {
			redisRateLimitHandler(duration, totalMaxCount, successMaxCount, limitKey, successKey)(c)
		} else {
			memoryRateLimitHandler(duration, totalMaxCount, successMaxCount, limitKey, successKey)(c)
		}
	}
}
