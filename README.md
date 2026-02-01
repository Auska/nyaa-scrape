# Nyaa 爬虫

一个用 Go 语言编写的简单爬虫，用于从 Nyaa 网站抓取种子信息并存储到 SQLite 数据库中。

## 功能

- 抓取 Nyaa 网站的种子信息（名称、磁力链接、分类、大小、日期）
- 将数据存储在 SQLite 数据库中
- 避免重复插入相同的种子
- 支持通过 CLI 参数或环境变量配置代理（HTTP/HTTPS/SOCKS5）
- 支持通过 CLI 参数进行灵活查询
- 支持将磁力链接直接发送到 Transmission 和 aria2 下载器
- 支持指定下载目录
- 支持 dry-run 预览模式
- 自动重试机制（最多 3 次）
- 批量插入优化

## 项目结构

```
nyaa-crawler/
├── cmd/                    # 应用程序入口
│   └── crawler/main.go     # 主程序入口点
├── internal/               # 内部包（仅本项目使用）
│   ├── crawler/            # 爬虫逻辑
│   │   └── crawler.go      # Config、Crawler、HTTP 请求、重试逻辑、页面解析
│   └── db/                 # 数据库操作
│       └── database.go     # DBService、CRUD 操作、索引管理
├── pkg/                    # 公共包（可被外部引用）
│   └── models/
│       └── torrent.go      # Torrent 数据结构
├── tools/
│   └── query_tool.go       # 查询工具，支持搜索和发送到下载器
├── configs/                # 配置文件目录（预留）
├── scripts/                # 脚本目录（预留）
├── test/                   # 测试数据目录（预留）
├── release/                # 发布构建输出目录
├── build.sh                # 构建脚本（支持 Linux/Windows/macOS）
├── main_test.go            # 测试文件
├── go.mod                  # Go 模块文件
├── go.sum                  # Go 模块校验和
├── README.md               # 本文件
├── USAGE.md                # 详细使用说明文档
└── AGENTS.md               # iFlow CLI 上下文配置
```

## 依赖

- Go 1.19 或更高版本
- SQLite3

### Go 依赖

- `github.com/PuerkitoBio/goquery v1.8.1` - HTML 解析
- `github.com/mattn/go-sqlite3 v1.14.16` - SQLite3 驱动
- `golang.org/x/net v0.7.0` - 网络库（SOCKS5 代理支持）

## 安装

1. 克隆此仓库：
   ```bash
   git clone <repository-url>
   cd nyaa-crawler
   ```

2. 初始化 Go 模块：
   ```bash
   go mod tidy
   ```

## 构建 Release 版本

项目包含一个构建脚本用于编译跨平台的二进制文件：

```bash
./build.sh
```

构建完成后，会在 `release` 目录中生成各平台的二进制文件和压缩包，可以直接运行而无需安装 Go 环境。

**构建产物**：
- `nyaa-crawler-linux-amd64` / `nyaa-query-linux-amd64`
- `nyaa-crawler-windows-amd64.exe` / `nyaa-query-windows-amd64.exe`
- `nyaa-crawler-macos-amd64` / `nyaa-query-macos-amd64`

## 快速开始

### 1. 运行爬虫

```bash
# 默认配置运行
go run ./cmd/crawler

# 使用 CLI 参数配置代理（推荐方式）
go run ./cmd/crawler -proxy socks5://proxy-server:port

# 使用环境变量配置代理（备用方式）
PROXY_URL=socks5://proxy-server:port go run ./cmd/crawler

# 指定数据库位置
go run ./cmd/crawler -db /path/to/custom.db

# 使用自定义URL
go run ./cmd/crawler -url https://nyaa.si/

# 完整示例
go run ./cmd/crawler -proxy socks5://proxy-server:port -url https://nyaa.si/ -db /path/to/custom.db
```

### 2. 查询数据

```bash
# 基本查询（显示最新 10 条）
go run ./tools/query_tool.go

# 文本搜索
go run ./tools/query_tool.go -regex "One Piece"

# 指定结果数量
go run ./tools/query_tool.go -limit 20

# 组合使用参数
go run ./tools/query_tool.go -db /path/to/custom.db -regex "One Piece" -limit 5
```

### 3. 发送到下载器

```bash
# 发送到 Transmission
go run ./tools/query_tool.go -regex "One Piece" -transmission "user:pass@http://localhost:9091/transmission/rpc"

# 发送到 aria2
go run ./tools/query_tool.go -regex "One Piece" -aria2 "token@http://localhost:6800/jsonrpc"

# 指定下载目录
go run ./tools/query_tool.go -regex "One Piece" -transmission "user:pass@http://localhost:9091/transmission/rpc" -download-dir /path/to/downloads

# 同时发送到多个下载器
go run ./tools/query_tool.go -regex "One Piece" -transmission "user:pass@http://localhost:9091/transmission/rpc" -aria2 "token@http://localhost:6800/jsonrpc"

# 预览模式（不实际发送）
go run ./tools/query_tool.go -regex "One Piece" -transmission "user:pass@http://localhost:9091/transmission/rpc" -dry-run
```

### 4. 直接查询数据库

```bash
# 查看所有种子数量
sqlite3 nyaa.db "SELECT COUNT(*) FROM torrents;"

# 查看前5个种子的信息
sqlite3 nyaa.db "SELECT id, name, category, size, date FROM torrents LIMIT 5;"

# 查找特定分类的种子
sqlite3 nyaa.db "SELECT id, name, size FROM torrents WHERE category LIKE '%Anime%' LIMIT 10;"
```

## 数据库结构

创建的 SQLite 数据库包含一个名为 `torrents` 的表，结构如下：

| 字段名 | 类型 | 描述 | 约束 |
|--------|------|------|------|
| `id` | INTEGER | 种子唯一标识 | PRIMARY KEY |
| `name` | TEXT | 种子名称 | - |
| `magnet` | TEXT | 磁力链接 | - |
| `category` | TEXT | 种子分类 | - |
| `size` | TEXT | 文件大小 | - |
| `date` | TEXT | 发布日期 | - |
| `pushed_to_transmission` | BOOLEAN | 是否已发送到 Transmission | DEFAULT 0 |
| `pushed_to_aria2` | BOOLEAN | 是否已发送到 aria2 | DEFAULT 0 |

### 索引

- `idx_torrents_name` - 名称索引
- `idx_torrents_category` - 分类索引
- `idx_torrents_date` - 日期索引

## 代理配置

爬虫支持多种方式配置代理服务器：

**方式 1：CLI 参数（推荐）**
```bash
go run ./cmd/crawler -proxy http://proxy-server:port
go run ./cmd/crawler -proxy https://proxy-server:port
go run ./cmd/crawler -proxy socks5://proxy-server:port
```

**方式 2：环境变量（备用）**
```bash
PROXY_URL=http://proxy-server:port go run ./cmd/crawler
PROXY_URL=socks5://proxy-server:port go run ./cmd/crawler
```

**优先级**：CLI 参数 > 环境变量 > 无代理

## 命令行参数

### 爬虫 (cmd/crawler/main.go)

| 参数 | 默认值 | 描述 |
|------|--------|------|
| `-db` | `./nyaa.db` | SQLite 数据库文件路径 |
| `-url` | `https://nyaa.si/` | 要抓取的 URL |
| `-proxy` | `""` | 代理服务器地址（http/https/socks5） |

### 查询工具 (tools/query_tool.go)

| 参数 | 默认值 | 描述 |
|------|--------|------|
| `-db` | `../nyaa.db` | SQLite 数据库文件路径 |
| `-regex` | `""` | 文本搜索模式（LIKE 操作符） |
| `-limit` | `10` | 返回结果数量 |
| `-transmission` | `""` | Transmission RPC URL |
| `-aria2` | `""` | aria2 JSON-RPC URL |
| `-download-dir` | `""` | 下载目录 |
| `-dry-run` | `false` | 预览模式，不实际发送 |

## 自定义

你可以修改代码来：

1. 抓取不同的页面或搜索结果
2. 更改数据库路径或表结构
3. 添加更多的错误处理或日志记录
4. 添加新的下载器支持

## 注意事项

1. 请遵守目标网站的 robots.txt 和服务条款
2. 不要过于频繁地请求，以免给服务器造成压力
3. 此代码仅用于学习和研究目的
4. 使用代理时请确保代理服务器的安全性

## 测试

```bash
# 运行所有测试
go test -v ./...

# 运行特定测试
go test -v -run TestNewDBService
```

## 许可证

MIT