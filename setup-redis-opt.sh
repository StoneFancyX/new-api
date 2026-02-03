#!/bin/bash

# Redis Docker Compose 安装脚本
# 安装位置: /opt/redis

set -e

echo "=========================================="
echo "Redis Docker Compose 安装脚本"
echo "安装位置: /opt/redis"
echo "=========================================="
echo ""

# 1. 创建目录结构
echo "📁 创建目录结构..."
sudo mkdir -p /opt/redis/{data,config,logs}

# 2. 设置目录权限（让当前用户可以访问）
echo "🔐 设置目录权限..."
sudo chown -R $(whoami):staff /opt/redis
chmod -R 755 /opt/redis

# 3. 创建 Redis 配置文件
echo "⚙️  创建 Redis 配置文件..."
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

# 客户端超时（0 表示禁用）
timeout 0

# TCP keepalive
tcp-keepalive 300

# 数据库数量
databases 16
EOF

# 4. 创建 docker-compose.yml
echo "🐳 创建 docker-compose.yml..."
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

# 5. 创建管理脚本
echo "🛠️  创建管理脚本..."
cat > /opt/redis/redis-ctl.sh << 'EOF'
#!/bin/bash

# Redis 管理脚本

cd /opt/redis

case "$1" in
  start)
    echo "🚀 启动 Redis..."
    docker-compose up -d
    echo "✅ Redis 已启动"
    ;;
  stop)
    echo "🛑 停止 Redis..."
    docker-compose stop
    echo "✅ Redis 已停止"
    ;;
  restart)
    echo "🔄 重启 Redis..."
    docker-compose restart
    echo "✅ Redis 已重启"
    ;;
  status)
    echo "📊 Redis 状态:"
    docker-compose ps
    ;;
  logs)
    echo "📝 Redis 日志:"
    docker-compose logs -f --tail=100
    ;;
  cli)
    echo "💻 进入 Redis CLI..."
    docker exec -it redis-standalone redis-cli
    ;;
  info)
    echo "ℹ️  Redis 信息:"
    docker exec -it redis-standalone redis-cli INFO
    ;;
  clean)
    echo "⚠️  清理所有数据（谨慎操作）"
    read -p "确认清理所有数据？(yes/no): " confirm
    if [ "$confirm" = "yes" ]; then
      docker-compose down
      rm -rf ./data/*
      rm -rf ./logs/*
      echo "✅ 数据已清理"
    else
      echo "❌ 取消操作"
    fi
    ;;
  backup)
    backup_dir="./backups/$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$backup_dir"
    echo "💾 备份数据到 $backup_dir..."
    docker exec redis-standalone redis-cli SAVE
    cp -r ./data/* "$backup_dir/"
    echo "✅ 备份完成"
    ;;
  *)
    echo "Redis 管理脚本"
    echo ""
    echo "用法: $0 {start|stop|restart|status|logs|cli|info|clean|backup}"
    echo ""
    echo "命令说明:"
    echo "  start   - 启动 Redis"
    echo "  stop    - 停止 Redis"
    echo "  restart - 重启 Redis"
    echo "  status  - 查看状态"
    echo "  logs    - 查看日志"
    echo "  cli     - 进入 Redis CLI"
    echo "  info    - 查看 Redis 信息"
    echo "  clean   - 清理所有数据"
    echo "  backup  - 备份数据"
    exit 1
    ;;
esac
EOF

chmod +x /opt/redis/redis-ctl.sh

# 6. 创建快捷命令（可选）
echo "🔗 创建快捷命令..."
cat > /opt/redis/README.md << 'EOF'
# Redis Docker Compose 配置

## 📁 目录结构

```
/opt/redis/
├── docker-compose.yml    # Docker Compose 配置
├── redis-ctl.sh          # 管理脚本
├── config/
│   └── redis.conf        # Redis 配置文件
├── data/                 # 数据目录（持久化）
├── logs/                 # 日志目录
└── backups/              # 备份目录
```

## 🚀 快速开始

### 启动 Redis
```bash
cd /opt/redis
./redis-ctl.sh start
```

### 查看状态
```bash
./redis-ctl.sh status
```

### 进入 Redis CLI
```bash
./redis-ctl.sh cli
```

## 📝 管理命令

```bash
./redis-ctl.sh start    # 启动 Redis
./redis-ctl.sh stop     # 停止 Redis
./redis-ctl.sh restart  # 重启 Redis
./redis-ctl.sh status   # 查看状态
./redis-ctl.sh logs     # 查看日志
./redis-ctl.sh cli      # 进入 Redis CLI
./redis-ctl.sh info     # 查看 Redis 信息
./redis-ctl.sh clean    # 清理所有数据
./redis-ctl.sh backup   # 备份数据
```

## 🔧 配置说明

### 端口
- Redis: 6379

### 数据持久化
- AOF: 开启（appendonly.aof）
- RDB: 开启（dump.rdb）
- 数据目录: /opt/redis/data

### 内存限制
- 最大内存: 512MB
- 淘汰策略: allkeys-lru

### 修改配置
编辑配置文件后需要重启：
```bash
vim /opt/redis/config/redis.conf
./redis-ctl.sh restart
```

## 🧪 测试连接

### 方式 1: 使用管理脚本
```bash
./redis-ctl.sh cli
PING
```

### 方式 2: 使用 docker exec
```bash
docker exec -it redis-standalone redis-cli ping
```

### 方式 3: 本地 redis-cli（如果已安装）
```bash
redis-cli -h localhost -p 6379 ping
```

## 📊 监控

### 查看实时日志
```bash
./redis-ctl.sh logs
```

### 查看 Redis 信息
```bash
./redis-ctl.sh info
```

### 查看内存使用
```bash
docker exec -it redis-standalone redis-cli INFO memory
```

### 查看所有 Key
```bash
docker exec -it redis-standalone redis-cli KEYS '*'
```

## 💾 备份与恢复

### 备份
```bash
./redis-ctl.sh backup
```

### 恢复
```bash
# 停止 Redis
./redis-ctl.sh stop

# 恢复数据
cp -r ./backups/YYYYMMDD_HHMMSS/* ./data/

# 启动 Redis
./redis-ctl.sh start
```

## 🔗 与项目集成

### 方式 1: 环境变量
```bash
export REDIS_CONN_STRING=redis://localhost:6379
```

### 方式 2: 修改项目配置
在 new-api 项目中使用：
```bash
cd /Users/mini/Workspace/new-api
export REDIS_CONN_STRING=redis://localhost:6379
go run main.go
```

## 🐛 故障排查

### 检查容器状态
```bash
docker ps | grep redis
```

### 查看详细日志
```bash
docker logs redis-standalone
```

### 检查端口占用
```bash
lsof -i :6379
```

### 重新构建
```bash
cd /opt/redis
docker-compose down
docker-compose up -d
```

## 🔒 安全建议

生产环境建议：
1. 设置密码（在 redis.conf 中添加 `requirepass your_password`）
2. 限制访问 IP（修改 bind 配置）
3. 定期备份数据
4. 监控内存使用
5. 设置合理的 maxmemory

## 📚 更多资源

- Redis 官方文档: https://redis.io/documentation
- Docker Compose 文档: https://docs.docker.com/compose/
EOF

echo ""
echo "=========================================="
echo "✅ 安装完成！"
echo "=========================================="
echo ""
echo "📁 安装位置: /opt/redis"
echo ""
echo "🚀 快速开始:"
echo "   cd /opt/redis"
echo "   ./redis-ctl.sh start"
echo ""
echo "📖 查看文档:"
echo "   cat /opt/redis/README.md"
echo ""
echo "🔗 添加到 PATH（可选）:"
echo "   echo 'export PATH=\"/opt/redis:\$PATH\"' >> ~/.zshrc"
echo "   source ~/.zshrc"
echo "   然后可以直接使用: redis-ctl.sh start"
echo ""
echo "=========================================="
