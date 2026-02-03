package model

import (
	"testing"
)

// TestTokenBeforeSave 测试 BeforeSave hook 验证
func TestTokenBeforeSave(t *testing.T) {
	tests := []struct {
		name    string
		token   Token
		wantErr bool
	}{
		{
			name: "有效配置",
			token: Token{
				RateLimitTotalCount:   100,
				RateLimitSuccessCount: 80,
				RateLimitDuration:     5,
			},
			wantErr: false,
		},
		{
			name: "总请求数为负数",
			token: Token{
				RateLimitTotalCount:   -1,
				RateLimitSuccessCount: 0,
				RateLimitDuration:     1,
			},
			wantErr: true,
		},
		{
			name: "总请求数超过上限",
			token: Token{
				RateLimitTotalCount:   10001,
				RateLimitSuccessCount: 100,
				RateLimitDuration:     1,
			},
			wantErr: true,
		},
		{
			name: "成功请求数超过总请求数",
			token: Token{
				RateLimitTotalCount:   100,
				RateLimitSuccessCount: 200,
				RateLimitDuration:     1,
			},
			wantErr: true,
		},
		{
			name: "限流周期超过上限",
			token: Token{
				RateLimitTotalCount:   100,
				RateLimitSuccessCount: 80,
				RateLimitDuration:     1441,
			},
			wantErr: true,
		},
		{
			name: "边界值测试 - 0",
			token: Token{
				RateLimitTotalCount:   0,
				RateLimitSuccessCount: 0,
				RateLimitDuration:     0,
			},
			wantErr: false,
		},
		{
			name: "边界值测试 - 最大值",
			token: Token{
				RateLimitTotalCount:   10000,
				RateLimitSuccessCount: 10000,
				RateLimitDuration:     1440,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.BeforeSave(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("BeforeSave() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
