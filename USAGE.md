# Nyaa 爬虫使用说明

## 项目结构

```
.
├── main.go       # 主爬虫程序
├── query_tool.go # 数据库查询工具
├── go.mod        # Go 模块定义
├── go.sum        # Go 模块校验和
├── test.html     # 测试用的 Nyaa 页面
├── nyaa.db       # SQLite 数据库文件
├── README.md     # 项目说明文档
├── USAGE.md      # 使用说明文档
└── tools/        # 工具目录
    └── query_tool.go # 查询工具
```

## 功能特性

1. 从 Nyaa 网站的 HTML 页面中提取种子信息
2. 解析种子的名称、磁力链接、分类、大小、日期等信息
3. 将数据存储在 SQLite 数据库中
4. 支持从本地 HTML 文件读取数据（用于测试）
5. 避免重复插入相同 ID 的种子
6. 支持通过环境变量配置代理

## 使用方法

### 1. 运行爬虫

```bash
go run main.go
```

爬虫会从Nyaa网站提取种子信息并存储到 `nyaa.db` 数据库中。

### 2. 使用代理运行爬虫

```bash
# 使用 HTTP 代理
PROXY_URL=http://proxy-server:port go run main.go

# 使用 HTTPS 代理
PROXY_URL=https://proxy-server:port go run main.go

# 使用 SOCKS5 代理
PROXY_URL=socks5://proxy-server:port go run main.go
```

### 3. 指定数据库位置

```bash
# 使用自定义数据库路径
go run main.go -db /path/to/custom.db

# 同时使用代理和自定义数据库
PROXY_URL=socks5://proxy-server:port go run main.go -db /path/to/custom.db
```

### 4. 查看帮助信息

```bash
go run main.go -help
```

## 查询工具使用方法

查询工具现在支持通过CLI参数进行灵活查询：

1. 基本查询：
   ```bash
   cd tools && go run query_tool.go
   ```

2. 指定数据库路径：
   ```bash
   cd tools && go run query_tool.go -db /path/to/database.db
   ```

3. 使用文本搜索（LIKE操作符）：
   ```bash
   cd tools && go run query_tool.go -regex "search_term"
   ```

4. 指定返回结果数量：
   ```bash
   cd tools && go run query_tool.go -limit 20
   ```

5. 组合使用多个参数：
   ```bash
   cd tools && go run query_tool.go -db ../custom.db -regex "One Piece" -limit 5
   ```

6. 查看帮助信息：
   ```bash
   cd tools && go run query_tool.go -help
   ```

## 发送到Transmission下载器

查询工具现在支持将磁力链接直接发送到Transmission下载器：

1. 发送搜索结果到Transmission：
   ```bash
   cd tools && go run query_tool.go -regex "One Piece" -transmission http://localhost:9091/transmission/rpc
   ```

2. 使用认证信息发送到Transmission（推荐方式）：
   ```bash
   cd tools && go run query_tool.go -regex "One Piece" -transmission "username:password@http://localhost:9091/transmission/rpc"
   ```

3. 预览将要发送的内容（不实际发送）：
   ```bash
   cd tools && go run query_tool.go -regex "One Piece" -transmission "username:password@http://localhost:9091/transmission/rpc" -dry-run
   ```

4. 程序会自动处理Transmission的CSRF保护机制（409错误和session-id），无需手动配置。

## 发送到aria2下载器

查询工具现在还支持将磁力链接直接发送到aria2下载器：

1. 发送搜索结果到aria2（使用令牌认证）：
   ```bash
   cd tools && go run query_tool.go -regex "One Piece" -aria2 "token@http://localhost:6800/jsonrpc"
   ```

2. 预览将要发送的内容（不实际发送）：
   ```bash
   cd tools && go run query_tool.go -regex "One Piece" -aria2 "token@http://localhost:6800/jsonrpc" -dry-run
   ```

3. 程序会自动从URL中提取令牌并进行认证。

## 同时发送到多个下载器

您还可以同时将磁力链接发送到Transmission和aria2：
   ```bash
   cd tools && go run query_tool.go -regex "One Piece" -transmission "username:password@http://localhost:9091/transmission/rpc" -aria2 "token@http://localhost:6800/jsonrpc"
   ```

5. 组合使用所有参数：
   ```bash
   cd tools && go run query_tool.go -db ../custom.db -regex "One Piece" -limit 5 -transmission "username:password@http://localhost:9091/transmission/rpc" -aria2 "token@http://localhost:6800/jsonrpc" -dry-run
   ```

### 3. 查询数据

```bash
cd tools && go run query_tool.go
```

查询工具会显示最新的10个种子，以及数据库的总体统计信息。

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

| 字段名     | 类型         | 描述         |
|------------|--------------|--------------|
| id         | INTEGER      | 种子ID       |
| name       | TEXT         | 种子名称     |
| magnet     | TEXT         | 磁力链接     |
| category   | TEXT         | 分类         |
| size       | TEXT         | 文件大小     |
| date       | TEXT         | 发布日期     |

## 代理配置

爬虫支持通过环境变量 `PROXY_URL` 配置代理服务器：

- HTTP/HTTPS 代理: `PROXY_URL=http://proxy-server:port`
- SOCKS5 代理: `PROXY_URL=socks5://proxy-server:port`

## 扩展功能

1. **修改数据源**：可以修改 `main.go` 中的代码来从实际的 Nyaa 网站抓取数据，而不是从本地文件读取。

2. **添加更多字段**：可以根据需要在数据库中添加更多字段来存储其他信息。

3. **定期更新**：可以设置定时任务来定期运行爬虫，以保持数据的更新。

4. **导出功能**：可以添加将数据导出为 CSV 或 JSON 格式的功能。

## 注意事项

1. 请遵守目标网站的 robots.txt 和服务条款
2. 不要过于频繁地请求，以免给服务器造成压力
3. 此代码仅用于学习和研究目的