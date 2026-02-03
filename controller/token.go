package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllTokens(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	tokens, err := model.GetAllUserTokens(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountUserTokens(userId)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tokens)
	common.ApiSuccess(c, pageInfo)
	return
}

func SearchTokens(c *gin.Context) {
	userId := c.GetInt("id")
	keyword := c.Query("keyword")
	token := c.Query("token")
	tokens, err := model.SearchUserTokens(userId, keyword, token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    tokens,
	})
	return
}

func GetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    token,
	})
	return
}

func GetTokenStatus(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	userId := c.GetInt("id")
	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"object":          "credit_summary",
		"total_granted":   token.RemainQuota,
		"total_used":      0, // not supported currently
		"total_available": token.RemainQuota,
		"expires_at":      expiredAt * 1000,
	})
}

func GetTokenUsage(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "No Authorization header",
		})
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid Bearer token",
		})
		return
	}
	tokenKey := parts[1]

	token, err := model.GetTokenByKey(strings.TrimPrefix(tokenKey, "sk-"), false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    true,
		"message": "ok",
		"data": gin.H{
			"object":               "token_usage",
			"name":                 token.Name,
			"total_granted":        token.RemainQuota + token.UsedQuota,
			"total_used":           token.UsedQuota,
			"total_available":      token.RemainQuota,
			"unlimited_quota":      token.UnlimitedQuota,
			"model_limits":         token.GetModelLimitsMap(),
			"model_limits_enabled": token.ModelLimitsEnabled,
			"expires_at":           expiredAt,
		},
	})
}

func AddToken(c *gin.Context) {
	token := model.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "令牌名称过长",
		})
		return
	}

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

	// 检查用户角色限制
	userId := c.GetInt("id")
	user, _ := model.GetUserById(userId, false)
	if user != nil && user.Role == common.RoleCommonUser {
		if token.RateLimitTotalCount > 1000 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "普通用户的令牌限流配置不能超过 1000 次/分钟",
			})
			return
		}
	}

	key, err := common.GenerateKey()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "生成令牌失败",
		})
		common.SysLog("failed to generate token key: " + err.Error())
		return
	}
	cleanToken := model.Token{
		UserId:                userId,
		Name:                  token.Name,
		Key:                   key,
		CreatedTime:           common.GetTimestamp(),
		AccessedTime:          common.GetTimestamp(),
		ExpiredTime:           token.ExpiredTime,
		RemainQuota:           token.RemainQuota,
		UnlimitedQuota:        token.UnlimitedQuota,
		ModelLimitsEnabled:    token.ModelLimitsEnabled,
		ModelLimits:           token.ModelLimits,
		AllowIps:              token.AllowIps,
		Group:                 token.Group,
		CrossGroupRetry:       token.CrossGroupRetry,
		RateLimitEnabled:      token.RateLimitEnabled,
		RateLimitTotalCount:   token.RateLimitTotalCount,
		RateLimitSuccessCount: token.RateLimitSuccessCount,
		RateLimitDuration:     token.RateLimitDuration,
	}

	// 配置合理性检查（记录警告但不阻止）
	if cleanToken.RateLimitTotalCount > 1000 {
		common.SysLog("High rate limit configured: token_name=" + cleanToken.Name + ", count=" + strconv.Itoa(cleanToken.RateLimitTotalCount))
	}

	err = cleanToken.Insert()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	err := model.DeleteTokenById(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func UpdateToken(c *gin.Context) {
	userId := c.GetInt("id")
	statusOnly := c.Query("status_only")
	token := model.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "令牌名称过长",
		})
		return
	}
	cleanToken, err := model.GetTokenByIds(token.Id, userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "令牌不存在或无权访问",
		})
		return
	}

	if token.Status == common.TokenStatusEnabled {
		if cleanToken.Status == common.TokenStatusExpired && cleanToken.ExpiredTime <= common.GetTimestamp() && cleanToken.ExpiredTime != -1 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "令牌已过期，无法启用，请先修改令牌过期时间，或者设置为永不过期",
			})
			return
		}
		if cleanToken.Status == common.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "令牌可用额度已用尽，无法启用，请先修改令牌剩余额度，或者设置为无限额度",
			})
			return
		}
	}

	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
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

		// 检查用户角色限制
		user, _ := model.GetUserById(userId, false)
		if user != nil && user.Role == common.RoleCommonUser {
			if token.RateLimitTotalCount > 1000 {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "普通用户的令牌限流配置不能超过 1000 次/分钟",
				})
				return
			}
		}

		// If you add more fields, please also update token.Update()
		cleanToken.Name = token.Name
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.RemainQuota = token.RemainQuota
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		cleanToken.ModelLimitsEnabled = token.ModelLimitsEnabled
		cleanToken.ModelLimits = token.ModelLimits
		cleanToken.AllowIps = token.AllowIps
		cleanToken.Group = token.Group
		cleanToken.CrossGroupRetry = token.CrossGroupRetry
		cleanToken.RateLimitEnabled = token.RateLimitEnabled
		cleanToken.RateLimitTotalCount = token.RateLimitTotalCount
		cleanToken.RateLimitSuccessCount = token.RateLimitSuccessCount
		cleanToken.RateLimitDuration = token.RateLimitDuration

		// 配置合理性检查（记录警告但不阻止）
		if cleanToken.RateLimitTotalCount > 1000 {
			common.SysLog("High rate limit configured: token_id=" + strconv.Itoa(cleanToken.Id) + ", count=" + strconv.Itoa(cleanToken.RateLimitTotalCount))
		}
	}
	err = cleanToken.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    cleanToken,
	})
	return
}

type TokenBatch struct {
	Ids []int `json:"ids"`
}

func DeleteTokenBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	userId := c.GetInt("id")
	count, err := model.BatchDeleteTokens(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}
