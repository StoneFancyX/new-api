# 在 /opt/redis 目录下使用 Docker Compose 安装 Redis

## 📁 目录结构

```
/opt/redis/
├── docker-compose.yml    # Docker Compose 配置
├── config/
│   └── redis.conf        # Redis 配置文件
├── data/                 # 数据目录（持久化）
└── logs/                 # 日志目录
```

## 🚀 安装步骤

### 1. 创建目录

```bash
sudo mkdir -p /opt/redis/{data,config,logs}
sudo chown -R $(whoami):staff /opt/redis
chmod -R 755 /opt/redis
```

### 2. 创建 Redis 配置文件

```bash
cat > /opt/redis/config/redis.conf << 'EOF'
# Redis 配置文件

# 绑定所有网络接口
bind 0.0.0.0

# 保护模式关闭（Docker 内部网络）
protected-mode no

# 端口
port 6379

# 数据持久化 - AOF
appendonly yes
appendfilename "appendonly.aof"

# 数据持久化 - RDB
save 900 1
save 300 10
save 60 10000

# 数据目录
dir /data

# 日志
loglevel notice
logfile /var/log/redis/redis.log

# 最大内存（根据需要调整）
maxmemory 512mb
maxmemory-policy allkeys-lru

# 慢查询日志
slowlog-log-slower-than 10000
slowlog-max-len 128

# TCP keepalive
tcp-keepalive 300

# 数据库数量
databases 16
EOF
```

### 3. 创建 docker-compose.yml

```bash
cat > /opt/redis/docker-compose.yml << 'EOF'
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    container_name: redis-standalone
    restart: unless-stopped
    ports:
      - "6379:6379"
    volumes:
      - ./data:/data
      - ./config/redis.conf:/usr/local/etc/redis/redis.conf
      - ./logs:/var/log/redis
    command: redis-server /usr/local/etc/redis/redis.conf
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 3
    networks:
      - redis-network

networks:
  redis-network:
    driver: bridge
EOF
```

### 4. 启动 Redis

```bash
cd /opt/redis
docker-compose up -d
```

### 5. 验证安装

```bash
# 查看容器状态
docker-compose ps

# 测试连接
docker exec -it redis-standalone redis-cli ping
# 应该返回: PONG
```

---

## 📝 常用管理命令

### 启动/停止/重启

```bash
cd /opt/redis

# 启动
docker-compose up -d

# 停止
docker-compose stop

# 重启
docker-compose restart

# 停止并删除容器
docker-compose down
```

### 查看状态和日志

```bash
# 查看容器状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 查看最近 100 行日志
docker-compose logs --tail=100
```

### 进入 Redis CLI

```bash
# 方式 1: 使用 docker-compose
docker-compose exec redis redis-cli

# 方式 2: 使用 docker
docker exec -it redis-standalone redis-cli

# 方式 3: 本地 redis-cli（如果已安装）
redis-cli -h localhost -p 6379
```

### Redis 常用命令

```bash
# 进入 Redis CLI 后
PING                          # 测试连接
KEYS *                        # 查看所有 Key
KEYS rateLimit:*              # 查看限流相关 Key
GET rateLimit:token:1         # 获取值
TTL rateLimit:token:1         # 查看过期时间
INFO                          # 查看服务器信息
INFO memory                   # 查看内存使用
DBSIZE                        # 查看 Key 数量
FLUSHALL                      # 清空所有数据（谨慎！）
```

---

## 🧪 与项目集成测试

### 1. 启动 Redis

```bash
cd /opt/redis
docker-compose up -d
```

### 2. 启动项目后端

```bash
cd /Users/mini/Workspace/new-api

# Redis 默认连接 localhost:6379，无需额外配置
go run main.go
```

### 3. 运行测试

```bash
# 使用测试脚本
./bin/rate_limit_test.sh localhost:3000 sk-your-token 15

# 检查 Redis 中的 Key
docker exec -it redis-standalone redis-cli
KEYS rateLimit:token:*
GET rateLimit:token:1
TTL rateLimit:token:1
exit
```

---

## 💾 数据备份与恢复

### 备份

```bash
cd /opt/redis

# 方式 1: 手动触发 RDB 快照
docker exec redis-standalone redis-cli SAVE

# 方式 2: 复制数据目录
backup_dir="./backups/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$backup_dir"
cp -r ./data/* "$backup_dir/"
echo "备份完成: $backup_dir"
```

### 恢复

```bash
cd /opt/redis

# 1. 停止 Redis
docker-compose stop

# 2. 恢复数据
cp -r ./backups/YYYYMMDD_HHMMSS/* ./data/

# 3. 启动 Redis
docker-compose up -d
```

---

## 🔧 配置优化

### 修改内存限制

编辑 `/opt/redis/config/redis.conf`：

```bash
# 修改最大内存（例如改为 1GB）
maxmemory 1gb
```

重启生效：

```bash
cd /opt/redis
docker-compose restart
```

### 修改持久化策略

编辑 `/opt/redis/config/redis.conf`：

```bash
# 更频繁的 RDB 快照
save 300 10
save 60 1000

# 或者只使用 AOF
appendonly yes
appendfsync everysec
```

---

## 🔍 监控和调试

### 实时监控所有命令

```bash
docker exec -it redis-standalone redis-cli MONITOR
```

### 查看慢查询

```bash
docker exec -it redis-standalone redis-cli SLOWLOG GET 10
```

### 查看客户端连接

```bash
docker exec -it redis-standalone redis-cli CLIENT LIST
```

### 查看内存使用详情

```bash
docker exec -it redis-standalone redis-cli INFO memory
```

---

## 🐛 常见问题

### Q1: 端口 6379 被占用？

**检查占用：**
```bash
lsof -i :6379
```

**解决方案：**
```bash
# 如果是本地 Redis，停止它
brew services stop redis

# 或者修改 docker-compose.yml 使用其他端口
ports:
  - "6380:6379"  # 改为 6380
```

### Q2: 权限问题？

```bash
# 确保目录权限正确
sudo chown -R $(whoami):staff /opt/redis
chmod -R 755 /opt/redis
```

### Q3: 数据丢失？

**原因：** 容器删除时没有使用数据卷

**解决：** 确保 docker-compose.yml 中配置了 volumes

### Q4: 容器启动失败？

```bash
# 查看详细日志
cd /opt/redis
docker-compose logs

# 检查配置文件语法
docker run --rm -v /opt/redis/config/redis.conf:/redis.conf redis:7-alpine redis-server /redis.conf --test-memory 1
```

---

## 📊 性能优化建议

### 开发环境

```yaml
# docker-compose.yml
services:
  redis:
    image: redis:7-alpine
    # 限制内存使用
    mem_limit: 512m
```

### 生产环境

1. **增加内存限制**
   ```bash
   maxmemory 2gb
   ```

2. **启用持久化**
   ```bash
   appendonly yes
   save 900 1
   ```

3. **设置密码**
   ```bash
   requirepass your_strong_password
   ```

4. **限制访问 IP**
   ```bash
   bind 127.0.0.1
   ```

---

## ✅ 快速参考

### 一键启动

```bash
cd /opt/redis && docker-compose up -d
```

### 一键停止

```bash
cd /opt/redis && docker-compose down
```

### 查看状态

```bash
cd /opt/redis && docker-compose ps
```

### 进入 CLI

```bash
docker exec -it redis-standalone redis-cli
```

### 查看日志

```bash
cd /opt/redis && docker-compose logs -f
```

---

## 🎯 下一步

Redis 安装完成后，可以：

1. **运行快速测试**: 参考 `docs/TESTING.md`
2. **完整测试**: 参考 `docs/token-rate-limit-test-plan.md`
3. **Docker 完整部署**: 参考 `docs/TESTING-DOCKER.md`

祝测试顺利！🎉
