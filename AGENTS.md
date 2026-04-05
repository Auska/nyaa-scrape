# AGENTS.md - Nyaa Crawler 项目配置

## 项目概述

- **项目名称**: nyaa-crawler
- **语言**: Go 1.19+
- **描述**: 从 Nyaa 种子网站抓取种子信息并存储到 PostgreSQL 数据库的工具

## 环境设置

Makefile 会自动设置 Go 环境变量。如需手动运行 Go 命令：

```bash
export PATH="/usr/local/go/bin:$PATH"
export PATH="$(go env GOPATH)/bin:$PATH"
```

## 常用命令

```bash
# 查看帮助
make help

# 运行爬虫
make run
# 或带参数运行
go run ./cmd/crawler -proxy socks5://proxy-server:port

# 运行查询工具
make query
# 或带参数运行
go run ./tools/query_tool.go -regex "One Piece" -limit 20

# 运行测试
make test

# 代码检查
make lint

# 格式化代码
make fmt

# 构建（所有平台）
make build

# 构建特定平台
make build-linux
make build-windows
make build-macos

# 清理构建产物
make clean
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
cmd/crawler/main.go       # 主程序入口
internal/crawler/         # 爬虫逻辑（依赖注入、HTTP 请求、重试）
internal/db/              # 数据库操作（实现 models.DBService 接口）
internal/downloader/      # 下载器客户端（Transmission、aria2）
pkg/models/               # 数据模型和接口定义
tools/query_tool.go       # 独立查询工具
```

### 核心组件

1. **Crawler** (`internal/crawler/crawler.go`)
   - 使用 Option 模式依赖注入
   - 支持 Context 取消
   - 批量插入优化
   - 预编译正则表达式

2. **DBService** (`internal/db/database.go`)
   - 实现 `models.DBService` 接口
   - 使用 `$1, $2` 占位符
   - `ON CONFLICT` 语法避免重复
   - 白名单验证防止 SQL 注入

3. **Downloader** (`internal/downloader/downloader.go`)
   - `TransmissionClient`: Transmission RPC 客户端
   - `Aria2Client`: aria2 JSON-RPC 客户端
   - 支持 HTTPClient 接口注入

4. **Models** (`pkg/models/`)
   - `Torrent`: 数据模型
   - `DBService`: 数据库接口

### 依赖注入

```go
// 创建 Crawler（使用 Option 模式）
c, err := crawler.NewCrawler(
    crawler.WithDB(dbs),
    crawler.WithProxy(proxyURL),
    crawler.WithMaxRetries(5),
)

// 创建 Transmission 客户端
client := downloader.NewTransmissionClient(httpClient, downloader.TransmissionConfig{
    URL:         url,
    User:        user,
    Password:    pass,
    DownloadDir: downloadDir,
})
```

### 数据流

```
CLI 参数 → Config 解析 → Crawler 初始化(依赖注入) → HTTP 请求 → goquery 解析 → DBService 写入
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
- defer 语句使用 `defer func() { _ = xxx.Close() }()` 忽略返回值（避免 errcheck 警告）
- 错误字符串不以大写字母开头
- 使用接口进行依赖注入，便于测试和扩展

## 设计原则

1. **依赖注入**: 通过 Option 模式注入依赖，便于测试和扩展
2. **接口隔离**: 定义 `models.DBService` 和 `downloader.HTTPClient` 接口
3. **单一职责**: 解析、下载、存储逻辑分离
4. **开闭原则**: 通过接口扩展，无需修改现有代码

## 许可证

MIT License
