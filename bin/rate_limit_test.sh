#!/bin/bash

# 令牌级别速率限制测试脚本
# 用法: ./rate_limit_test.sh <domain> <token> <count>

if [ $# -lt 3 ]; then
  echo "Usage: rate_limit_test.sh <domain> <token> <count>"
  echo "Example: ./rate_limit_test.sh localhost:3000 sk-xxx 15"
  exit 1
fi

domain=$1
token=$2
count=$3

echo "=========================================="
echo "令牌级别速率限制测试"
echo "=========================================="
echo "Domain: $domain"
echo "Token: ${token:0:10}..."
echo "Request count: $count"
echo "=========================================="
echo ""

success_count=0
rate_limited_count=0

for ((i=1; i<=count; i++)); do
  echo -n "Request $i: "

  result=$(curl -s -w "\n%{http_code}" \
           http://"$domain"/v1/chat/completions \
           -H "Content-Type: application/json" \
           -H "Authorization: Bearer $token" \
           -d '{
             "messages": [{"content": "hi", "role": "user"}],
             "model": "gpt-3.5-turbo",
             "stream": false,
             "max_tokens": 1
           }')

  http_code=$(echo "$result" | tail -n1)

  if [ "$http_code" = "200" ]; then
    echo "✅ Success (200)"
    ((success_count++))
  elif [ "$http_code" = "429" ]; then
    echo "🚫 Rate Limited (429)"
    ((rate_limited_count++))
    # 显示错误消息
    error_msg=$(echo "$result" | head -n-1 | grep -o '"message":"[^"]*"' | cut -d'"' -f4)
    echo "   Message: $error_msg"
  else
    echo "❌ Error ($http_code)"
  fi

  # 短暂延迟避免过快
  sleep 0.1
done

echo ""
echo "=========================================="
echo "测试结果汇总"
echo "=========================================="
echo "总请求数: $count"
echo "成功: $success_count"
echo "被限流: $rate_limited_count"
echo "=========================================="
