# AGENTS.md - Nyaa Crawler 项目配置

## 项目概述

- **项目名称**: nyaa-crawler
- **语言**: Go 1.19+
- **描述**: 从 Nyaa 种子网站抓取种子信息并存储到 SQLite 数据库的工具
- **功能特性**:
  - 抓取 Nyaa 网站的种子信息（名称、磁力链接、分类、大小、日期）
  - 支持批量插入优化，避免重复插入
  - 支持代理（HTTP/HTTPS/SOCKS5）通过 CLI 参数或环境变量配置
  - 支持将磁力链接发送到 Transmission 和 aria2 下载器
  - 支持指定下载目录
  - 支持 dry-run 预览模式
  - 自动重试机制（最多 3 次）

## 项目结构

遵循 Go 标准项目布局：

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
├── USAGE.md                # 详细使用说明文档
└── AGENTS.md               # 本文件，用于 iFlow CLI 上下文配置
```

## 数据库

- **类型**: SQLite3
- **路径**: 默认 `./nyaa.db`（可通过 `-db` 参数自定义）
- **连接池配置**:
  - 最大打开连接数: 25
  - 最大空闲连接数: 5
  - 连接最大存活时间: 5 分钟

### 表结构

**torrents**: 存储种子信息

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

## 核心功能

### 爬虫功能
- 从 Nyaa 网站抓取种子信息
- 支持 HTTP/HTTPS/SOCKS5 代理（CLI 参数或环境变量）
- 避免重复插入相同 ID 的种子（使用 `INSERT OR IGNORE`）
- CLI 参数支持自定义 URL 和数据库路径
- 自动重试机制（最多 3 次，指数退避）
- HTTP 请求超时设置（30 秒）
- 支持从本地 HTML 文件解析（用于测试）

### 查询功能
- 独立的查询工具，支持文本搜索和结果限制
- LIKE 模式匹配搜索
- 按时间倒序排列结果
- 显示数据库统计信息（总数、磁力链接数、匹配数）
- 表格化输出格式

### 下载器集成
- 支持 Transmission RPC API（自动处理 409 CSRF 错误）
- 支持 aria2 JSON-RPC API
- 支持指定下载目录
- 支持 dry-run 预览模式
- 自动更新推送状态（避免重复发送）
- 支持同时发送到多个下载器

### 数据库优化
- 批量插入优化（使用事务）
- 连接池管理
- 多字段索引加速查询
- 统计查询优化

## 主要命令

### 运行爬虫

```bash
# 默认配置运行
go run ./cmd/crawler

# 使用 CLI 参数配置代理（推荐方式）
go run ./cmd/crawler -proxy socks5://proxy-server:port

# 使用环境变量配置代理（备用方式）
PROXY_URL=socks5://proxy-server:port go run ./cmd/crawler

# 自定义 URL 和数据库路径
go run ./cmd/crawler -url https://nyaa.si/ -db /path/to/custom.db

# 完整示例：使用代理 + 自定义 URL + 自定义数据库
go run ./cmd/crawler -proxy socks5://proxy-server:port -url https://nyaa.si/ -db /path/to/custom.db
```

### 查询工具

```bash
# 基本查询（显示最新 10 条）
go run ./tools/query_tool.go

# 文本搜索
go run ./tools/query_tool.go -regex "One Piece"

# 指定结果数量
go run ./tools/query_tool.go -limit 20

# 发送到 Transmission（不带下载目录）
go run ./tools/query_tool.go -regex "One Piece" -transmission "user:pass@http://localhost:9091/transmission/rpc"

# 发送到 Transmission（指定下载目录）
go run ./tools/query_tool.go -regex "One Piece" -transmission "user:pass@http://localhost:9091/transmission/rpc" -download-dir /path/to/downloads

# 发送到 aria2（不带下载目录）
go run ./tools/query_tool.go -regex "One Piece" -aria2 "token@http://localhost:6800/jsonrpc"

# 发送到 aria2（指定下载目录）
go run ./tools/query_tool.go -regex "One Piece" -aria2 "token@http://localhost:6800/jsonrpc" -download-dir /path/to/downloads

# 同时发送到多个下载器
go run ./tools/query_tool.go -regex "One Piece" -transmission "user:pass@http://localhost:9091/transmission/rpc" -aria2 "token@http://localhost:6800/jsonrpc" -download-dir /path/to/downloads

# 预览模式（不实际发送）
go run ./tools/query_tool.go -regex "One Piece" -transmission "user:pass@http://localhost:9091/transmission/rpc" -download-dir /path/to/downloads -dry-run
```

### 运行测试

```bash
# 运行所有测试
go test -v ./...

# 运行特定测试
go test -v -run TestNewDBService
```

### 构建 Release 版本

```bash
./build.sh
```

构建完成后，二进制文件和压缩包位于 `release/` 目录：

**二进制文件**:
- `nyaa-crawler-linux-amd64` - Linux 爬虫程序
- `nyaa-query-linux-amd64` - Linux 查询工具
- `nyaa-crawler-windows-amd64.exe` - Windows 爬虫程序
- `nyaa-query-windows-amd64.exe` - Windows 查询工具
- `nyaa-crawler-macos-amd64` - macOS 爬虫程序
- `nyaa-query-macos-amd64` - macOS 查询工具

**压缩包**:
- `nyaa-crawler-linux-amd64-YYYYMMDD.tar.gz`
- `nyaa-crawler-windows-amd64-YYYYMMDD.tar.gz`
- `nyaa-crawler-macos-amd64-YYYYMMDD.tar.gz`

## 架构设计

### 核心组件

#### 1. Crawler (internal/crawler/crawler.go)
负责网络请求和页面解析：
- `Config`: 配置结构体，包含数据库路径、URL、代理设置
- `Crawler`: 爬虫核心结构体
  - `Client`: HTTP 客户端（支持代理）
  - `DBS`: 数据库服务引用
  - `MaxRetries`: 最大重试次数
- `fetchWithRetry()`: 带重试的 HTTP 请求
- `ScrapePage()`: 从 URL 抓取页面
- `parseTorrentRow()`: 解析单个种子行

#### 2. DBService (internal/db/database.go)
负责数据库操作：
- `NewDBService()`: 创建数据库连接和表结构
- `InsertTorrent()`: 插入单个种子
- `InsertTorrents()`: 批量插入种子（事务）
- `GetLatestTorrents()`: 获取最新种子
- `GetTorrentsByPattern()`: 按模式搜索种子
- `UpdatePushedStatus()`: 更新推送状态
- 连接池配置：MaxOpenConns=25, MaxIdleConns=5

#### 3. Torrent Model (pkg/models/torrent.go)
数据模型：
- ID: 种子唯一标识
- Name, Magnet, Category, Size, Date: 基本信息
- PushedToTransmission, PushedToAria2: 推送状态

#### 4. Query Tool (tools/query_tool.go)
独立的命令行查询工具：
- 支持搜索和筛选
- 支持 Transmission 和 aria2 集成
- 支持 dry-run 模式
- 表格化输出

### 数据流

```
用户输入 (CLI参数)
    ↓
Config 解析
    ↓
Crawler 初始化 (创建 HTTP Client + DB 连接)
    ↓
fetchWithRetry (HTTP 请求 + 重试)
    ↓
goquery 解析 HTML
    ↓
parseTorrentRow (提取数据)
    ↓
DBService.InsertTorrent (写入数据库)
    ↓
Query Tool 查询 + 发送到下载器
```

### 代理支持

- HTTP/HTTPS 代理：通过 `http.ProxyURL` 配置
- SOCKS5 代理：通过 `golang.org/x/net/proxy` 包配置
- 优先级：`-proxy` CLI 参数 > `PROXY_URL` 环境变量 > 无代理

## 测试

### 运行测试

```bash
# 运行所有测试
go test -v ./...

# 运行特定测试
go test -v -run TestNewDBService

# 查看测试覆盖率
go test -cover ./...
```

### 测试文件

- `main_test.go`: 主测试文件
- 包含单元测试和集成测试

## 调试

### 查看日志

程序使用 `log.Printf` 输出日志，包括：
- 配置信息（数据库路径、URL、代理设置）
- 抓取进度（插入的种子数量）
- 错误信息（HTTP 错误、数据库错误）
- 查询结果统计

### 调试技巧

1. **测试爬虫**：使用小范围的 URL 或本地 HTML 文件
2. **测试代理**：使用 `curl` 测试代理连通性
3. **查看数据库**：使用 `sqlite3 nyaa.db` 直接查询
4. **dry-run 模式**：在发送到下载器前使用 `-dry-run` 预览

## 依赖

### Go 依赖

- `github.com/PuerkitoBio/goquery v1.8.1` - HTML 解析
- `github.com/mattn/go-sqlite3 v1.14.16` - SQLite3 驱动
- `golang.org/x/net v0.7.0` - 网络库（SOCKS5 代理支持）
- `github.com/andybalholm/cascadia v1.3.1` - CSS 选择器（goquery 间接依赖）

### 系统依赖

- Go 1.19 或更高版本
- SQLite3（如需直接查询数据库）

## 常见问题

### Q: 如何检查数据库中是否有数据？
```bash
sqlite3 nyaa.db "SELECT COUNT(*) FROM torrents;"
```

### Q: 代理连接失败怎么办？
1. 检查代理 URL 格式是否正确（`http://`, `https://`, `socks5://`）
2. 确认代理服务是否可用
3. 尝试使用其他协议的代理
4. 不使用代理直接连接测试

### Q: Transmission 返回 409 错误？
程序会自动处理 Transmission 的 CSRF 保护（409 错误），无需手动干预。如果仍然失败，检查：
- RPC URL 是否正确
- 用户名密码是否正确
- Transmission 服务是否运行

### Q: aria2 返回认证错误？
确认：
- aria2 RPC URL 正确（默认 `http://localhost:6800/jsonrpc`）
- 令牌配置正确（在 aria2.conf 中设置 `rpc-secret`）
- aria2 启用了 RPC 功能

### Q: 种子重复插入？
使用 `INSERT OR IGNORE` 语句，相同 ID 的种子不会被重复插入。如果需要更新，修改为 `INSERT OR REPLACE`。

### Q: 如何只抓取特定分类的种子？
修改 `crawler.go` 中的 `parseTorrentRow` 方法，添加分类过滤逻辑。

## 故障排除

### 网络问题
- 检查网络连接
- 尝试使用代理
- 检查目标网站是否可访问
- 查看 HTTP 状态码错误日志

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

## 环境变量

- `PROXY_URL`: 代理服务器地址（HTTP/HTTPS/SOCKS5）

**注意**: 代理配置优先级：CLI 参数 `-proxy` > 环境变量 `PROXY_URL` > 无代理。推荐使用 CLI 参数，更加明确和易调试。

## 开发规范

### 代码规范

- 使用英文提交信息（遵循 conventional commits 格式：`feat:`, `fix:`, `refactor:` 等）
- 遵循 Go 代码风格（使用 `gofmt` 格式化）
- 错误处理使用 `log.Printf` 而非直接 panic
- 公共 API 放在 `pkg/` 目录
- 内部实现放在 `internal/` 目录
- 应用程序入口放在 `cmd/` 目录
- CLI 参数使用 `flag` 包定义

### 包结构

- **pkg/models**: 可复用的数据模型（如 `Torrent` 结构体）
- **internal/db**: 数据库操作逻辑（DBService、CRUD 操作）
- **internal/crawler**: 爬虫核心逻辑（HTTP 请求、重试、页面解析）
- **cmd/crawler**: 主程序入口点
- **tools**: 独立工具程序（如 query_tool.go）

## 未来计划

### 功能增强
- [ ] 支持更多种子站点
- [ ] 支持分页抓取（抓取多页数据）
- [ ] 支持定时任务配置
- [ ] 添加种子去重策略（基于名称或内容）
- [ ] 支持导出为 CSV/JSON 格式
- [ ] 添加 Web UI 界面
- [ ] 支持多数据库后端（PostgreSQL、MySQL）

### 性能优化
- [ ] 并发抓取多个页面
- [ ] 实现增量更新机制
- [ ] 添加缓存层
- [ ] 优化数据库查询

### 代码质量
- [ ] 增加单元测试覆盖率
- [ ] 添加集成测试
- [ ] 添加 CI/CD 流程
- [ ] 添加代码质量检查（golangci-lint）

### 安全性
- [ ] 添加配置文件加密支持
- [ ] 改进代理认证机制
- [ ] 添加请求限流保护

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
