# Nyaa Crawler

从 [Nyaa](https://nyaa.si) 种子网站抓取种子信息并存储到 PostgreSQL 数据库的命令行工具，同时支持通过 Transmission 和 aria2 推送磁力链接。

## 功能

- 抓取 Nyaa 种子信息（名称、磁力链接、分类、大小、日期）
- 存储到 PostgreSQL，自动去重
- 支持 HTTP/HTTPS/SOCKS5 代理
- 按正则表达式或最新顺序查询种子
- 推送磁力链接到 Transmission 或 aria2
- 支持 `--dry-run` 预览模式
- 跨平台构建（Linux/macOS/Windows amd64）

## 环境要求

- Go 1.19+
- PostgreSQL

## 快速开始

```bash
# 查看帮助
make help

# 运行爬虫
make run

# 带代理运行
go run ./cmd/crawler -proxy socks5://proxy-server:port

# 运行查询工具
make query

# 按正则查询
go run ./cmd/query -regex "One Piece" -limit 20
```

## 常用命令

| 命令 | 说明 |
|------|------|
| `make run` | 运行爬虫 |
| `make query` | 运行查询工具 |
| `make test` | 运行测试（含 race 检测和覆盖率） |
| `make lint` | 运行 golangci-lint |
| `make fmt` | 格式化代码 |
| `make build` | 构建所有平台 |
| `make build-linux` | 构建 Linux amd64 |
| `make build-windows` | 构建 Windows amd64 |
| `make build-macos` | 构建 macOS amd64 |
| `make clean` | 清理构建产物 |

## 环境变量

| 变量 | 描述 |
|------|------|
| `NYAA_DB` | PostgreSQL 连接字符串（如 `postgres://user:pass@localhost:5432/nyaa?sslmode=disable`） |
| `NYAA_PROXY` | 代理地址（http/https/socks5） |

优先级：CLI 参数 > 环境变量 > 默认值

## 数据库

- 类型：PostgreSQL
- 默认连接：`postgres://localhost:5432/nyaa?sslmode=disable`
- 驱动：`github.com/lib/pq`

### 表结构 — torrents

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

## 项目结构

```
cmd/crawler/main.go       # 爬虫主程序入口
cmd/query/main.go         # 查询工具入口
internal/crawler/         # 爬虫逻辑（依赖注入、HTTP 请求、重试）
internal/db/              # 数据库操作（实现 models.DBService 接口）
internal/downloader/      # 下载器客户端（Transmission、aria2）
pkg/models/               # 数据模型和接口定义
tools/                    # 辅助脚本
```

### 核心组件

- **Crawler** (`internal/crawler/crawler.go`) — Option 模式依赖注入，支持 Context 取消，批量插入优化
- **DBService** (`internal/db/database.go`) — 实现 `models.DBService` 接口，`ON CONFLICT` 避免重复，白名单验证防 SQL 注入
- **Downloader** (`internal/downloader/downloader.go`) — Transmission RPC 和 aria2 JSON-RPC 客户端
- **Models** (`pkg/models/`) — `Torrent` 数据模型与 `DBService` 接口定义

### 数据流

```
CLI 参数 → Config 解析 → Crawler 初始化(依赖注入) → HTTP 请求 → goquery 解析 → DBService 写入
```

查询/推送流程：

```
Query 工具 → DBService 读取种子 → Downloader 推送磁力链接 → 更新推送状态
```

## 依赖

| 依赖 | 用途 |
|------|------|
| `github.com/PuerkitoBio/goquery` | HTML 解析 |
| `github.com/lib/pq` | PostgreSQL 驱动 |
| `golang.org/x/net` | SOCKS5 代理支持 |

## 开发规范

- 使用英文提交信息（conventional commits: `feat:`, `fix:`, `refactor:`）
- 使用 `gofmt` 格式化代码
- 错误处理使用 `log.Printf` 而非 panic
- PostgreSQL 使用 `$1, $2` 占位符，`ON CONFLICT` 语法
- defer 语句使用 `defer func() { _ = xxx.Close() }()` 忽略返回值
- 错误字符串不以大写字母开头
- 使用接口进行依赖注入，便于测试和扩展

## 许可证

MIT License
