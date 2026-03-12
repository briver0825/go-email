# Email Demo

基于 Go + docker-mailserver 的邮件收取和查看系统。通过 IMAP 轮询拉取邮件，存入 SQLite，提供 Web UI 浏览。

## 项目结构

```
email-demo/
├── main.go                # 入口
├── config.go              # 环境变量配置
├── Dockerfile             # Go 应用镜像
├── docker-compose.yml     # 编排：mailserver + Go app
├── setup.sh               # 创建邮箱账号脚本
├── store/store.go         # SQLite 存储层
├── fetcher/fetcher.go     # IMAP 轮询器
├── api/server.go          # HTTP API
├── web/index.html         # Web UI
└── docker-data/           # mailserver 配置挂载目录
```

---

## 一、本地开发（不用 Docker）

适合调试 Go 代码，需要已有一个可用的 IMAP 邮箱。

### 1. 安装依赖

```bash
go mod download
```

### 2. 设置环境变量

```bash
export IMAP_HOST=imap.gmail.com        # 你的 IMAP 服务器
export IMAP_PORT=993
export IMAP_USER=you@gmail.com
export IMAP_PASS=your-app-password     # Gmail 需使用应用专用密码
export IMAP_TLS=true
export DB_PATH=./emails.db
export ATTACHMENT_DIR=./attachments
```

### 3. 运行

```bash
go run .
```

### 4. 访问

打开浏览器：http://localhost:8080

---

## 二、Docker Compose 部署（推荐）

完整部署 docker-mailserver（收发邮件）+ Go 应用（IMAP 轮询 + Web UI）。

### 1. 修改配置

编辑 `docker-compose.yml`，替换以下内容为你的实际域名和账号：

```yaml
# mailserver 服务
hostname: mail.yourdomain.com
OVERRIDE_HOSTNAME: mail.yourdomain.com

# email-app 服务
IMAP_USER: user@yourdomain.com
IMAP_PASS: your-secure-password
```

### 2. 启动 mailserver

```bash
docker compose up -d mailserver
```

等待约 10-20 秒让 mailserver 完全启动。

### 3. 创建邮箱账号

```bash
# 参数必须与 docker-compose.yml 中的 IMAP_USER / IMAP_PASS 一致
bash setup.sh user@yourdomain.com your-secure-password
```

### 4. 启动全部服务

```bash
docker compose up -d
```

### 5. 访问 Web UI

打开浏览器：http://localhost:8080

### 6. 查看日志

```bash
# 全部日志
docker compose logs -f

# 只看 Go 应用
docker compose logs -f email-app

# 只看邮件服务器
docker compose logs -f mailserver
```

### 7. 停止

```bash
docker compose down
```

数据保存在 Docker volumes 中，重启不丢失。彻底清除数据：

```bash
docker compose down -v
```

---

## 三、DNS 配置（接收外部邮件）

要从 Gmail 等外部邮箱接收邮件，服务器需要正确的 DNS 记录：

| 类型 | 名称 | 值 | 说明 |
|------|------|----|------|
| A | mail.yourdomain.com | 你的服务器 IP | 邮件服务器地址 |
| MX | yourdomain.com | mail.yourdomain.com (优先级 10) | 邮件路由 |
| TXT | yourdomain.com | `v=spf1 mx -all` | SPF 防伪造 |
| PTR | 你的服务器 IP | mail.yourdomain.com | 反向解析（联系 VPS 提供商设置） |

### TLS 证书（生产环境必须）

修改 `docker-compose.yml` 中 mailserver 的 SSL 配置：

```yaml
environment:
  - SSL_TYPE=letsencrypt
volumes:
  - /etc/letsencrypt:/etc/letsencrypt:ro   # 挂载证书
```

同时将 `IMAP_TLS` 改为 `true`，`IMAP_PORT` 改为 `993`。

---

## 四、环境变量一览

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `IMAP_HOST` | mailserver | IMAP 服务器地址 |
| `IMAP_PORT` | 143 | IMAP 端口 |
| `IMAP_USER` | user@example.com | IMAP 登录用户 |
| `IMAP_PASS` | password | IMAP 登录密码 |
| `IMAP_TLS` | false | 是否使用 TLS |
| `IMAP_MAILBOX` | INBOX | 拉取的邮箱文件夹 |
| `HTTP_ADDR` | :8080 | Web UI 监听地址 |
| `DB_PATH` | /data/emails.db | SQLite 数据库路径 |
| `ATTACHMENT_DIR` | /data/attachments | 附件保存目录 |
| `POLL_INTERVAL` | 30 | 轮询间隔（秒） |

---

## 五、测试邮件收取

### 方法 A：用 swaks 发送测试邮件

```bash
# 安装 swaks (macOS)
brew install swaks

# 发送测试邮件到 mailserver
swaks --to user@yourdomain.com \
      --from test@test.com \
      --server localhost:25 \
      --header "Subject: Test Email" \
      --body "Hello from swaks!"
```

### 方法 B：用 telnet 手动发送

```bash
telnet localhost 25
HELO test
MAIL FROM:<test@test.com>
RCPT TO:<user@yourdomain.com>
DATA
Subject: Hello

This is a test email.
.
QUIT
```

发送后等待最多 30 秒（轮询间隔），刷新 Web UI 即可看到邮件。

---

## 六、常见问题

**Q: email-app 启动后一直报 IMAP 连接失败？**
A: mailserver 启动较慢，等待 30 秒后 email-app 会自动重试。也可检查账号是否已创建：
```bash
docker exec mailserver setup email list
```

**Q: 如何添加更多邮箱账号？**
```bash
docker exec mailserver setup email add another@yourdomain.com password123
```

**Q: 如何修改轮询频率？**
修改 `docker-compose.yml` 中 `POLL_INTERVAL` 的值（单位：秒），然后重启：
```bash
docker compose restart email-app
```

**Q: 数据存在哪里？**
- 邮件原始数据：Docker volume `maildata`
- SQLite 数据库和附件：Docker volume `appdata`
- 查看 volume：`docker volume ls`
