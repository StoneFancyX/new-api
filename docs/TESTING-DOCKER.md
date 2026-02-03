# 使用 Docker 测试令牌级别速率限制

## 🐳 方案 1: 使用 Docker Compose（推荐）

### 步骤 1: 构建本地镜像

由于我们修改了代码，需要先构建本地镜像：

```bash
# 在项目根目录
cd /Users/mini/Workspace/new-api

# 构建 Docker 镜像
docker build -t new-api:feature-rate-limit .
```

### 步骤 2: 修改 docker-compose.yml

临时修改 docker-compose.yml 使用本地镜像：

```bash
# 备份原文件
cp docker-compose.yml docker-compose.yml.bak

# 修改第 19 行，将：
# image: calciumion/new-api:latest
# 改为：
# image: new-api:feature-rate-limit
```

或者使用命令：

```bash
sed -i.bak 's|image: calciumion/new-api:latest|image: new-api:feature-rate-limit|' docker-compose.yml
```

### 步骤 3: 启动服务

```bash
# 启动所有服务（PostgreSQL + Redis + New-API）
docker-compose up -d

# 查看日志
docker-compose logs -f new-api

# 等待服务启动（约 30 秒）
# 看到 "Server started on port 3000" 表示启动成功
```

### 步骤 4: 访问系统

打开浏览器访问：http://localhost:3000

### 步骤 5: 测试

按照之前的测试步骤进行测试：

1. **前端测试** - 创建/编辑令牌，查看速率限制配置
2. **API 测试** - 使用测试脚本
3. **Redis 检查** - 进入 Redis 容器检查

```bash
# 进入 Redis 容器
docker exec -it redis redis-cli

# 在 Redis CLI 中执行
KEYS rateLimit:token:*
GET rateLimit:token:1
TTL rateLimit:token:1
```

### 步骤 6: 停止服务

```bash
# 停止所有服务
docker-compose down

# 恢复原配置
mv docker-compose.yml.bak docker-compose.yml
```

---

## 🚀 方案 2: 仅使用 Docker 运行 Redis（更快）

如果您想更快地测试，可以只用 Docker 运行 Redis，本地运行后端：

```bash
# 1. 启动 Redis
docker run -d --name redis-test -p 6379:6379 redis:latest

# 2. 本地运行后端
go run main.go

# 3. 本地运行前端
cd web && bun run dev

# 4. 测试完成后清理
docker stop redis-test
docker rm redis-test
```

---

## 🧪 完整测试流程（Docker 方式）

### 1. 构建并启动

```bash
# 构建镜像
docker build -t new-api:feature-rate-limit .

# 修改 docker-compose.yml
sed -i.bak 's|image: calciumion/new-api:latest|image: new-api:feature-rate-limit|' docker-compose.yml

# 启动服务
docker-compose up -d

# 查看日志，确认启动成功
docker-compose logs -f new-api
```

### 2. 前端测试

1. 访问 http://localhost:3000
2. 登录（默认账号需要先注册）
3. 进入令牌管理
4. 创建令牌，启用速率限制
5. 配置：10次/1分钟

### 3. API 测试

```bash
# 使用测试脚本
./bin/rate_limit_test.sh localhost:3000 sk-your-token 15

# 预期结果：
# - 前 10 次：✅ Success (200)
# - 后 5 次：🚫 Rate Limited (429)
```

### 4. Redis 检查

```bash
# 进入 Redis 容器
docker exec -it redis redis-cli

# 查看限流 Key
KEYS rateLimit:token:*

# 查看计数
GET rateLimit:token:1
GET rateLimit:token:1:success

# 查看 TTL
TTL rateLimit:token:1

# 退出
exit
```

### 5. 查看日志

```bash
# 查看后端日志
docker-compose logs new-api | grep -E "(rate|limit|token)"

# 应该看到：
# - High rate limit configured: token_id=1, count=10
# - Cleaned rate limit keys for token 1 (deleted)
```

### 6. 清理

```bash
# 停止服务
docker-compose down

# 恢复配置
mv docker-compose.yml.bak docker-compose.yml

# 删除测试镜像（可选）
docker rmi new-api:feature-rate-limit
```

---

## 🔍 调试技巧

### 查看容器状态

```bash
# 查看所有容器
docker-compose ps

# 查看特定容器日志
docker-compose logs -f new-api
docker-compose logs -f redis
docker-compose logs -f postgres
```

### 进入容器

```bash
# 进入 new-api 容器
docker exec -it new-api sh

# 查看数据库文件
ls -la /data

# 查看日志
ls -la /app/logs
```

### 检查数据库

```bash
# 进入 PostgreSQL 容器
docker exec -it postgres psql -U root -d new-api

# 查看表结构
\d tokens

# 查看令牌数据
SELECT id, name, rate_limit_enabled, rate_limit_total_count FROM tokens;

# 退出
\q
```

### 重新构建

如果修改了代码，需要重新构建：

```bash
# 停止服务
docker-compose down

# 重新构建镜像
docker build -t new-api:feature-rate-limit .

# 启动服务
docker-compose up -d
```

---

## ⚡ 快速测试命令（一键执行）

```bash
# 创建测试脚本
cat > test-docker.sh << 'EOF'
#!/bin/bash
set -e

echo "🐳 开始 Docker 测试..."

# 1. 构建镜像
echo "📦 构建镜像..."
docker build -t new-api:feature-rate-limit .

# 2. 修改配置
echo "⚙️  修改配置..."
sed -i.bak 's|image: calciumion/new-api:latest|image: new-api:feature-rate-limit|' docker-compose.yml

# 3. 启动服务
echo "🚀 启动服务..."
docker-compose up -d

# 4. 等待服务启动
echo "⏳ 等待服务启动（30秒）..."
sleep 30

# 5. 检查服务状态
echo "✅ 检查服务状态..."
docker-compose ps

echo ""
echo "🎉 服务已启动！"
echo "📝 访问地址: http://localhost:3000"
echo "🔍 查看日志: docker-compose logs -f new-api"
echo "🛑 停止服务: docker-compose down"
echo ""
EOF

chmod +x test-docker.sh

# 运行测试
./test-docker.sh
```

---

## 📊 测试清单

使用 Docker 测试时，确保验证：

- [ ] Docker 镜像构建成功
- [ ] 所有容器正常启动（new-api, redis, postgres）
- [ ] 数据库自动迁移成功（查看日志）
- [ ] 前端可以访问（http://localhost:3000）
- [ ] 可以创建令牌并配置速率限制
- [ ] API 限流功能正常工作
- [ ] Redis Key 正确生成和清理
- [ ] 日志中没有错误信息

---

## 🐛 常见问题

### Q1: 构建镜像失败？
**A:** 检查 Dockerfile 和网络连接，确保可以下载依赖

### Q2: 容器启动失败？
**A:** 查看日志：`docker-compose logs new-api`

### Q3: 端口被占用？
**A:** 修改 docker-compose.yml 中的端口映射：
```yaml
ports:
  - "3001:3000"  # 改为 3001
```

### Q4: 数据库连接失败？
**A:** 确保 PostgreSQL 容器已启动：`docker-compose ps postgres`

### Q5: Redis 连接失败？
**A:** 确保 Redis 容器已启动：`docker-compose ps redis`
