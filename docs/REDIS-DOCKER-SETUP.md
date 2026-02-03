# 在 Mac 上使用 Docker 安装 Redis

## 🚀 方法 1: 快速启动（推荐用于测试）

最简单的方式，适合快速测试：

```bash
# 启动 Redis 容器
docker run -d \
  --name redis \
  -p 6379:6379 \
  redis:latest

# 验证 Redis 是否运行
docker ps | grep redis

# 测试连接
docker exec -it redis redis-cli ping
# 应该返回: PONG
```

### 管理命令

```bash
# 停止 Redis
docker stop redis

# 启动 Redis
docker start redis

# 重启 Redis
docker restart redis

# 查看日志
docker logs redis

# 删除容器（会丢失数据）
docker rm -f redis
```

---

## 💾 方法 2: 带数据持久化（推荐用于开发）

如果需要保存数据，使用数据卷：

```bash
# 创建数据目录（推荐位置）
mkdir -p ~/docker-data/redis

# 启动 Redis 并挂载数据卷
docker run -d \
  --name redis \
  -p 6379:6379 \
  -v ~/docker-data/redis:/data \
  redis:latest redis-server --appendonly yes

# 验证数据持久化
docker exec -it redis redis-cli
# 在 Redis CLI 中执行：
SET test "hello"
GET test
exit

# 重启容器后数据仍然存在
docker restart redis
docker exec -it redis redis-cli GET test
# 应该返回: "hello"
```

### 数据目录说明

- **推荐位置**: `~/docker-data/redis`
- **为什么选这里**:
  - 在用户主目录下，权限管理简单
  - 容易备份和迁移
  - 不会影响系统目录
  - 符合 macOS 最佳实践

### 数据管理

```bash
# 查看数据文件
ls -lh ~/docker-data/redis/

# 备份数据
cp -r ~/docker-data/redis ~/docker-data/redis-backup-$(date +%Y%m%d)

# 清空数据（谨慎操作）
docker stop redis
rm -rf ~/docker-data/redis/*
docker start redis
```

---

## 🎯 方法 3: 使用项目的 docker-compose（最推荐）

项目已经配置好了 Redis，直接使用：

```bash
# 在项目根目录
cd /Users/mini/Workspace/new-api

# 只启动 Redis（不启动其他服务）
docker-compose up -d redis

# 查看 Redis 状态
docker-compose ps redis

# 查看 Redis 日志
docker-compose logs -f redis

# 停止 Redis
docker-compose stop redis

# 删除 Redis 容器
docker-compose down redis
```

### 优势

- ✅ 配置已经优化好
- ✅ 与项目其他服务集成
- ✅ 统一管理所有容器
- ✅ 网络配置自动完成

---

## 🔧 Redis 配置优化（可选）

如果需要自定义 Redis 配置：

### 1. 创建配置文件

```bash
# 创建配置目录
mkdir -p ~/docker-data/redis-config

# 创建配置文件
cat > ~/docker-data/redis-config/redis.conf << 'EOF'
# Redis 配置文件

# 绑定所有网络接口（Docker 内部使用）
bind 0.0.0.0

# 保护模式关闭（Docker 内部网络安全）
protected-mode no

# 端口
port 6379

# 数据持久化
appendonly yes
appendfilename "appendonly.aof"

# RDB 快照
save 900 1
save 300 10
save 60 10000

# 日志级别
loglevel notice

# 最大内存（根据需要调整）
maxmemory 256mb
maxmemory-policy allkeys-lru

# 慢查询日志
slowlog-log-slower-than 10000
slowlog-max-len 128
EOF
```

### 2. 使用自定义配置启动

```bash
docker run -d \
  --name redis \
  -p 6379:6379 \
  -v ~/docker-data/redis:/data \
  -v ~/docker-data/redis-config/redis.conf:/usr/local/etc/redis/redis.conf \
  redis:latest redis-server /usr/local/etc/redis/redis.conf
```

---

## 🧪 完整测试流程

### 1. 启动 Redis

```bash
# 使用方法 2（带持久化）
mkdir -p ~/docker-data/redis

docker run -d \
  --name redis \
  -p 6379:6379 \
  -v ~/docker-data/redis:/data \
  redis:latest redis-server --appendonly yes

# 等待 2 秒
sleep 2

# 验证启动
docker ps | grep redis
```

### 2. 测试连接

```bash
# 方式 1: 使用 docker exec
docker exec -it redis redis-cli ping

# 方式 2: 如果本地安装了 redis-cli
redis-cli ping

# 方式 3: 使用 telnet
telnet localhost 6379
# 输入: PING
# 应该返回: +PONG
```

### 3. 启动项目后端

```bash
# 在项目根目录
cd /Users/mini/Workspace/new-api

# 确保 Redis 连接配置正确（默认就是 localhost:6379）
# 启动后端
go run main.go

# 查看日志，确认 Redis 连接成功
# 应该看到: Redis connection established
```

### 4. 运行测试

```bash
# 使用测试脚本
./bin/rate_limit_test.sh localhost:3000 sk-your-token 15

# 检查 Redis 中的 Key
docker exec -it redis redis-cli
KEYS rateLimit:token:*
GET rateLimit:token:1
TTL rateLimit:token:1
exit
```

---

## 📊 Redis 监控和调试

### 查看 Redis 信息

```bash
# 进入 Redis CLI
docker exec -it redis redis-cli

# 查看服务器信息
INFO

# 查看内存使用
INFO memory

# 查看所有 Key
KEYS *

# 查看特定模式的 Key
KEYS rateLimit:*

# 查看 Key 的类型
TYPE rateLimit:token:1

# 查看 Key 的 TTL
TTL rateLimit:token:1

# 查看 Key 的值
GET rateLimit:token:1

# 删除 Key
DEL rateLimit:token:1

# 清空所有数据（谨慎！）
FLUSHALL

# 退出
exit
```

### 实时监控

```bash
# 监控所有命令
docker exec -it redis redis-cli MONITOR

# 查看慢查询
docker exec -it redis redis-cli SLOWLOG GET 10

# 查看客户端连接
docker exec -it redis redis-cli CLIENT LIST
```

---

## 🔍 常见问题

### Q1: 端口 6379 被占用？

**检查占用：**
```bash
lsof -i :6379
```

**解决方案 1 - 停止占用进程：**
```bash
# 如果是本地 Redis
brew services stop redis

# 如果是其他进程，找到 PID 后
kill -9 <PID>
```

**解决方案 2 - 使用其他端口：**
```bash
docker run -d \
  --name redis \
  -p 6380:6379 \
  redis:latest

# 修改项目配置使用 6380 端口
# 或设置环境变量：
export REDIS_CONN_STRING=redis://localhost:6380
```

### Q2: 容器启动失败？

```bash
# 查看详细日志
docker logs redis

# 检查容器状态
docker ps -a | grep redis

# 删除旧容器重新创建
docker rm -f redis
# 然后重新运行启动命令
```

### Q3: 数据丢失？

**原因：** 没有使用数据卷

**解决：** 使用方法 2 或方法 3，确保数据持久化

### Q4: 连接被拒绝？

```bash
# 检查容器是否运行
docker ps | grep redis

# 检查端口映射
docker port redis

# 测试网络连接
telnet localhost 6379

# 检查防火墙（macOS 一般不需要）
```

### Q5: 性能问题？

```bash
# 检查内存使用
docker exec -it redis redis-cli INFO memory

# 检查慢查询
docker exec -it redis redis-cli SLOWLOG GET 10

# 增加内存限制
docker run -d \
  --name redis \
  -p 6379:6379 \
  -m 512m \
  redis:latest
```

---

## 🎯 推荐配置总结

### 开发环境（推荐）

```bash
# 1. 创建数据目录
mkdir -p ~/docker-data/redis

# 2. 启动 Redis
docker run -d \
  --name redis \
  --restart unless-stopped \
  -p 6379:6379 \
  -v ~/docker-data/redis:/data \
  redis:latest redis-server --appendonly yes

# 3. 验证
docker exec -it redis redis-cli ping
```

### 生产环境（使用 docker-compose）

```bash
# 使用项目的 docker-compose.yml
cd /Users/mini/Workspace/new-api
docker-compose up -d redis
```

---

## 📝 快速参考

### 常用命令速查

```bash
# 启动
docker start redis

# 停止
docker stop redis

# 重启
docker restart redis

# 查看日志
docker logs -f redis

# 进入 CLI
docker exec -it redis redis-cli

# 查看状态
docker ps | grep redis

# 删除容器
docker rm -f redis

# 备份数据
docker exec redis redis-cli SAVE
cp -r ~/docker-data/redis ~/backup/
```

### Redis CLI 常用命令

```bash
PING                    # 测试连接
KEYS *                  # 查看所有 Key
GET key                 # 获取值
SET key value           # 设置值
DEL key                 # 删除 Key
TTL key                 # 查看过期时间
FLUSHALL                # 清空所有数据
INFO                    # 查看服务器信息
```

---

## ✅ 下一步

Redis 安装完成后，可以：

1. **运行测试**: 参考 `docs/TESTING.md`
2. **Docker 完整测试**: 参考 `docs/TESTING-DOCKER.md`
3. **查看测试计划**: 参考 `docs/token-rate-limit-test-plan.md`

祝测试顺利！🎉
