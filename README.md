# Nyaa 爬虫

一个用 Go 语言编写的简单爬虫，用于从 Nyaa 网站抓取种子信息并存储到 SQLite 数据库中。

## 功能

- 抓取 Nyaa 网站的种子信息（名称、磁力链接、分类、大小、日期）
- 将数据存储在 SQLite 数据库中
- 避免重复插入相同的种子
- 支持通过环境变量配置代理

## 依赖

- Go 1.19 或更高版本
- SQLite3

## 安装

1. 克隆此仓库：
   ```
   git clone <repository-url>
   cd nyaa-crawler
   ```

2. 初始化 Go 模块：
   ```
   go mod tidy
   ```

## 使用方法

1. 运行爬虫：
   ```
   go run main.go
   ```

2. 使用代理运行爬虫：
   ```
   # HTTP/HTTPS 代理
   PROXY_URL=http://proxy-server:port go run main.go
   
   # SOCKS5 代理
   PROXY_URL=socks5://proxy-server:port go run main.go
   ```

3. 指定数据库位置：
   ```
   go run main.go -db /path/to/custom.db
   ```

4. 同时使用代理和自定义数据库：
   ```
   PROXY_URL=socks5://proxy-server:port go run main.go -db /path/to/custom.db
   ```

5. 查看帮助信息：
   ```
   go run main.go -help
   ```

6. 查看数据库中的数据：
   ```
   sqlite3 nyaa.db "SELECT * FROM torrents LIMIT 10;"
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

6. 查看查询工具帮助信息：
   ```bash
   cd tools && go run query_tool.go -help
   ```

## 发送到Transmission下载器

查询工具现在支持将磁力链接直接发送到Transmission下载器：

1. 发送搜索结果到Transmission：
   ```bash
   cd tools && go run query_tool.go -regex "One Piece" -send -transmission http://localhost:9091/transmission/rpc
   ```

2. 使用认证信息发送到Transmission（推荐方式）：
   ```bash
   cd tools && go run query_tool.go -regex "One Piece" -send -transmission "username:password@http://localhost:9091/transmission/rpc"
   ```

3. 预览将要发送的内容（不实际发送）：
   ```bash
   cd tools && go run query_tool.go -regex "One Piece" -send -transmission "username:password@http://localhost:9091/transmission/rpc" -dry-run
   ```

4. 程序会自动处理Transmission的CSRF保护机制（409错误和session-id），无需手动配置。

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

## 自定义

你可以修改 `main.go` 中的代码来：

1. 抓取不同的页面或搜索结果
2. 更改数据库路径或表结构
3. 添加更多的错误处理或日志记录

## 注意事项

1. 请遵守目标网站的 robots.txt 和服务条款
2. 不要过于频繁地请求，以免给服务器造成压力
3. 此代码仅用于学习和研究目的

## 许可证

MIT