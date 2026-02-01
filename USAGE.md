# Nyaa 爬虫使用说明

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
├── README.md               # 项目说明文档
├── USAGE.md                # 本文件
└── AGENTS.md               # iFlow CLI 上下文配置
```

## 功能特性

1. 从 Nyaa 网站的 HTML 页面中提取种子信息
2. 解析种子的名称、磁力链接、分类、大小、日期等信息
3. 将数据存储在 SQLite 数据库中
4. 支持从本地 HTML 文件读取数据（用于测试）
5. 避免重复插入相同 ID 的种子（使用 `INSERT OR IGNORE`）
6. 支持通过 CLI 参数或环境变量配置代理（HTTP/HTTPS/SOCKS5）
7. 支持将磁力链接发送到 Transmission 和 aria2 下载器
8. 支持指定下载目录
9. 支持 dry-run 预览模式
10. 自动重试机制（最多 3 次，指数退避）
11. 批量插入优化（使用事务）

## 爬虫使用方法

### 1. 基本用法

```bash
# 默认配置运行
go run ./cmd/crawler
```

爬虫会从 Nyaa 网站提取种子信息并存储到 `nyaa.db` 数据库中。

### 2. 使用自定义 URL

```bash
# 使用自定义 URL
go run ./cmd/crawler -url https://example.com/nyaa-clone

# 同时使用自定义 URL 和数据库路径
go run ./cmd/crawler -url https://example.com/nyaa-clone -db /path/to/custom.db
```

### 3. 使用代理

**方式 1：CLI 参数（推荐）**
```bash
# 使用 HTTP 代理
go run ./cmd/crawler -proxy http://proxy-server:port

# 使用 HTTPS 代理
go run ./cmd/crawler -proxy https://proxy-server:port

# 使用 SOCKS5 代理
go run ./cmd/crawler -proxy socks5://proxy-server:port
```

**方式 2：环境变量（备用）**
```bash
# 使用 HTTP 代理
PROXY_URL=http://proxy-server:port go run ./cmd/crawler

# 使用 HTTPS 代理
PROXY_URL=https://proxy-server:port go run ./cmd/crawler

# 使用 SOCKS5 代理
PROXY_URL=socks5://proxy-server:port go run ./cmd/crawler
```

**优先级**：CLI 参数 > 环境变量 > 无代理

### 4. 指定数据库位置

```bash
# 使用自定义数据库路径
go run ./cmd/crawler -db /path/to/custom.db

# 同时使用代理和自定义数据库
go run ./cmd/crawler -proxy socks5://proxy-server:port -db /path/to/custom.db
```

### 5. 完整示例

```bash
# 使用代理 + 自定义 URL + 自定义数据库
go run ./cmd/crawler -proxy socks5://proxy-server:port -url https://nyaa.si/ -db /path/to/custom.db
```

### 6. 查看帮助信息

```bash
go run ./cmd/crawler -help
```

## 查询工具使用方法

### 1. 基本查询

```bash
# 基本查询（显示最新 10 条）
go run ./tools/query_tool.go
```

查询工具会显示最新的 10 个种子，以及数据库的总体统计信息。

### 2. 指定数据库路径

```bash
go run ./tools/query_tool.go -db /path/to/database.db
```

### 3. 文本搜索

```bash
# 使用文本搜索（LIKE 操作符）
go run ./tools/query_tool.go -regex "One Piece"

# 搜索特定关键词
go run ./tools/query_tool.go -regex "1080p"
```

### 4. 指定返回结果数量

```bash
# 指定返回结果数量
go run ./tools/query_tool.go -limit 20
```

### 5. 组合使用多个参数

```bash
# 组合使用多个参数
go run ./tools/query_tool.go -db /path/to/custom.db -regex "One Piece" -limit 5
```

### 6. 查看帮助信息

```bash
go run ./tools/query_tool.go -help
```

## 发送到 Transmission 下载器

查询工具支持将磁力链接直接发送到 Transmission 下载器。

### 1. 基本用法

```bash
# 发送搜索结果到 Transmission（无需认证）
go run ./tools/query_tool.go -regex "One Piece" -transmission "http://localhost:9091/transmission/rpc"
```

### 2. 使用认证信息（推荐）

```bash
# 发送搜索结果到 Transmission（使用用户名密码认证）
go run ./tools/query_tool.go -regex "One Piece" -transmission "username:password@http://localhost:9091/transmission/rpc"
```

### 3. 指定下载目录

```bash
# 发送到 Transmission 并指定下载目录
go run ./tools/query_tool.go -regex "One Piece" -transmission "username:password@http://localhost:9091/transmission/rpc" -download-dir /path/to/downloads
```

### 4. 预览模式

```bash
# 预览将要发送的内容（不实际发送）
go run ./tools/query_tool.go -regex "One Piece" -transmission "username:password@http://localhost:9091/transmission/rpc" -dry-run
```

### 5. 注意事项

- 程序会自动处理 Transmission 的 CSRF 保护机制（409 错误和 session-id），无需手动配置
- 只有未发送过的磁力链接会被发送（基于 `pushed_to_transmission` 字段）
- 发送成功后会自动更新数据库中的推送状态

## 发送到 aria2 下载器

查询工具支持将磁力链接直接发送到 aria2 下载器。

### 1. 基本用法

```bash
# 发送搜索结果到 aria2（使用令牌认证）
go run ./tools/query_tool.go -regex "One Piece" -aria2 "token@http://localhost:6800/jsonrpc"
```

### 2. 指定下载目录

```bash
# 发送到 aria2 并指定下载目录
go run ./tools/query_tool.go -regex "One Piece" -aria2 "token@http://localhost:6800/jsonrpc" -download-dir /path/to/downloads
```

### 3. 预览模式

```bash
# 预览将要发送的内容（不实际发送）
go run ./tools/query_tool.go -regex "One Piece" -aria2 "token@http://localhost:6800/jsonrpc" -dry-run
```

### 4. 注意事项

- 程序会自动从 URL 中提取令牌并进行认证
- 只有未发送过的磁力链接会被发送（基于 `pushed_to_aria2` 字段）
- 发送成功后会自动更新数据库中的推送状态

## 同时发送到多个下载器

您还可以同时将磁力链接发送到 Transmission 和 aria2。

### 1. 基本用法

```bash
# 同时发送到 Transmission 和 aria2
go run ./tools/query_tool.go -regex "One Piece" -transmission "username:password@http://localhost:9091/transmission/rpc" -aria2 "token@http://localhost:6800/jsonrpc"
```

### 2. 指定下载目录

```bash
# 同时发送到多个下载器并指定下载目录
go run ./tools/query_tool.go -regex "One Piece" -transmission "username:password@http://localhost:9091/transmission/rpc" -aria2 "token@http://localhost:6800/jsonrpc" -download-dir /path/to/downloads
```

### 3. 组合使用所有参数

```bash
# 组合使用所有参数
go run ./tools/query_tool.go -db /path/to/custom.db -regex "One Piece" -limit 5 -transmission "username:password@http://localhost:9091/transmission/rpc" -aria2 "token@http://localhost:6800/jsonrpc" -download-dir /path/to/downloads -dry-run
```

## 直接查询数据库

您可以使用 SQLite3 命令行工具直接查询数据库：

```bash
# 查看所有种子数量
sqlite3 nyaa.db "SELECT COUNT(*) FROM torrents;"

# 查看前5个种子的信息
sqlite3 nyaa.db "SELECT id, name, category, size, date FROM torrents LIMIT 5;"

# 查找特定分类的种子
sqlite3 nyaa.db "SELECT id, name, size FROM torrents WHERE category LIKE '%Anime%' LIMIT 10;"

# 查看已发送到 Transmission 的种子
sqlite3 nyaa.db "SELECT id, name FROM torrents WHERE pushed_to_transmission = 1;"

# 查看已发送到 aria2 的种子
sqlite3 nyaa.db "SELECT id, name FROM torrents WHERE pushed_to_aria2 = 1;"
```

## 数据库结构

创建的 SQLite 数据库包含一个名为 `torrents` 的表，结构如下：

| 字段名 | 类型 | 描述 | 约束 |
|--------|------|------|------|
| `id` | INTEGER | 种子唯一标识 | PRIMARY KEY |
| `name` | TEXT | 种子名称 | - |
| `magnet` | TEXT | 磁力链接 | - |
| `category` | TEXT | 种子分类 | - |
| `size` | TEXT | 文件大小（如 "1.2 GiB"） | - |
| `date` | TEXT | 发布日期 | - |
| `pushed_to_transmission` | BOOLEAN | 是否已发送到 Transmission | DEFAULT 0 |
| `pushed_to_aria2` | BOOLEAN | 是否已发送到 aria2 | DEFAULT 0 |

### 索引

- `idx_torrents_name` - 名称索引，加速按名称搜索
- `idx_torrents_category` - 分类索引，加速按分类筛选
- `idx_torrents_date` - 日期索引，加速按日期排序查询

## 代理配置

爬虫支持多种方式配置代理服务器：

### 支持的代理类型

- HTTP 代理: `http://proxy-server:port`
- HTTPS 代理: `https://proxy-server:port`
- SOCKS5 代理: `socks5://proxy-server:port`

### 配置方式

**方式 1：CLI 参数（推荐）**
```bash
go run ./cmd/crawler -proxy http://proxy-server:port
go run ./cmd/crawler -proxy socks5://proxy-server:port
```

**方式 2：环境变量（备用）**
```bash
PROXY_URL=http://proxy-server:port go run ./cmd/crawler
PROXY_URL=socks5://proxy-server:port go run ./cmd/crawler
```

**优先级**：CLI 参数 > 环境变量 > 无代理

## 命令行参数速查

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

## 扩展功能

1. **修改数据源**：可以修改 `internal/crawler/crawler.go` 中的代码来从实际的 Nyaa 网站抓取数据，或者添加对其他网站的支持。

2. **添加更多字段**：可以根据需要在数据库中添加更多字段来存储其他信息（如种子状态、下载次数等）。

3. **定期更新**：可以设置定时任务（如 cron）来定期运行爬虫，以保持数据的更新。

4. **导出功能**：可以添加将数据导出为 CSV 或 JSON 格式的功能。

5. **批量操作**：可以添加批量删除、批量标记已读等功能。

## 故障排除

### 网络问题

- 检查网络连接是否正常
- 尝试使用代理
- 检查目标网站是否可访问
- 查看日志中的 HTTP 状态码错误

### 代理问题

- 确认代理 URL 格式是否正确
- 确认代理服务是否可用
- 尝试使用其他协议的代理
- 不使用代理直接连接测试

### 数据库问题

- 确认数据库文件权限
- 检查磁盘空间
- 尝试删除数据库文件重新创建
- 使用 `sqlite3` 直接查询验证

### 下载器集成问题

- 确认下载器服务正在运行
- 检查 RPC 端点 URL 是否正确
- 使用 `-dry-run` 模式测试
- 查看错误日志获取详细信息

## 注意事项

1. 请遵守目标网站的 robots.txt 和服务条款
2. 不要过于频繁地请求，以免给服务器造成压力
3. 此代码仅用于学习和研究目的
4. 使用代理时请确保代理服务器的安全性
5. 定期备份数据库文件以防数据丢失

## 许可证

MIT