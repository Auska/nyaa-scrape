# AGENTS.md - Nyaa Crawler 项目配置

## 项目概述

- **项目名称**: nyaa-crawler
- **语言**: Go 1.19+
- **描述**: 从 Nyaa 种子网站抓取种子信息并存储到 PostgreSQL 数据库的工具

## 常用命令

```bash
# 运行爬虫
go run ./cmd/crawler

# 使用代理运行
go run ./cmd/crawler -proxy socks5://proxy-server:port

# 查询工具
go run ./tools/query_tool.go
go run ./tools/query_tool.go -regex "One Piece" -limit 20

# 运行测试
go test -v ./...
go test -v -run TestNewDBService

# 构建
./build.sh
```

## 环境变量

| 变量 | 描述 |
|------|------|
| `NYAA_DB` | PostgreSQL 连接字符串（如 `postgres://user:pass@localhost:5432/nyaa?sslmode=disable`） |
| `NYAA_PROXY` | 代理地址（http/https/socks5） |

**优先级**: CLI 参数 > 环境变量 > 默认值

## 数据库

- **类型**: PostgreSQL
- **默认连接**: `postgres://localhost:5432/nyaa?sslmode=disable`
- **驱动**: `github.com/lib/pq`

### 表结构

**torrents**:
| 字段 | 类型 | 描述 |
|------|------|------|
| `id` | INTEGER | 种子唯一标识 (PRIMARY KEY) |
| `name` | TEXT | 种子名称 |
| `magnet` | TEXT | 磁力链接 |
| `category` | TEXT | 种子分类 |
| `size` | TEXT | 文件大小 |
| `date` | TEXT | 发布日期 |
| `pushed_to_transmission` | BOOLEAN | 是否已发送到 Transmission |
| `pushed_to_aria2` | BOOLEAN | 是否已发送到 aria2 |

## 架构

```
cmd/crawler/main.go      # 主程序入口
internal/crawler/        # 爬虫逻辑（HTTP 请求、重试、页面解析）
internal/db/             # 数据库操作（DBService、CRUD）
pkg/models/              # 数据模型（Torrent 结构体）
tools/query_tool.go      # 独立查询工具（搜索、发送到下载器）
```

### 核心组件

1. **Crawler** (`internal/crawler/crawler.go`): 网络请求、HTML 解析、重试逻辑
2. **DBService** (`internal/db/database.go`): PostgreSQL 操作，使用 `$1, $2` 占位符
3. **Query Tool** (`tools/query_tool.go`): 搜索种子，发送到 Transmission/aria2

### 数据流

```
CLI 参数 → Config 解析 → Crawler 初始化 → HTTP 请求 → goquery 解析 → DBService 写入
```

## 依赖

- `github.com/PuerkitoBio/goquery` - HTML 解析
- `github.com/lib/pq` - PostgreSQL 驱动
- `golang.org/x/net` - SOCKS5 代理支持

## 开发规范

- 使用英文提交信息（conventional commits: `feat:`, `fix:`, `refactor:`）
- 使用 `gofmt` 格式化代码
- 错误处理使用 `log.Printf` 而非 panic
- PostgreSQL 使用 `$1, $2` 占位符，`ON CONFLICT` 语法

## 许可证

MIT License
