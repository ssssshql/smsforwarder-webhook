# SmsCodeWebhook

短信验证码 Webhook 服务：接收短信推送、从正文提取验证码、按 `type` + `phone` 缓存，供客户端拉取。

- 框架：Go + Gin
- 存储：内存（进程重启清空）
- 每个 `(type, phone)` 仅保留最近一条，默认 5 分钟过期

## 目录结构

```
.
├── main.go
├── internal/
│   ├── auth/       # 密钥加载、HMAC 签名
│   ├── handler/    # HTTP 接口
│   ├── parse/      # content 解析、验证码提取
│   └── store/      # 内存仓库
├── Dockerfile
└── docker-compose.yml
```

## 快速开始

### Docker 一键启动（推荐）

镜像：[`ssssshql/smsforwarder-webhook`](https://hub.docker.com/r/ssssshql/smsforwarder-webhook)

```bash
docker run -d \
  --name smsforwarder-webhook \
  --restart unless-stopped \
  -p 8080:8080 \
  -v smsforwarder_data:/data \
  -e CODE_TTL=5m \
  -e TZ=Asia/Shanghai \
  ssssshql/smsforwarder-webhook:latest
```

查看首次生成的密钥：

```bash
docker logs smsforwarder-webhook
docker exec smsforwarder-webhook cat /data/api_key
```

也可在启动时指定密钥：

```bash
docker run -d \
  --name smsforwarder-webhook \
  --restart unless-stopped \
  -p 8080:8080 \
  -e API_KEY=your-16char-key \
  -e CODE_TTL=5m \
  ssssshql/smsforwarder-webhook:latest
```

### Docker Compose

```yaml
services:
  smsforwarder-webhook:
    image: ssssshql/smsforwarder-webhook:latest
    container_name: smsforwarder-webhook
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      # API_KEY: "your-secret-key"   # 可选，不设则首次自动生成
      PORT: "8080"
      DATA_DIR: /data
      CODE_TTL: 5m
      TZ: Asia/Shanghai
    volumes:
      - smsforwarder_data:/data

volumes:
  smsforwarder_data:
```

```bash
docker compose up -d
docker compose logs -f
docker compose exec smsforwarder-webhook cat /data/api_key
```

### 本地源码运行

```bash
go run .
# 或
go build -o sms-code-webhook.exe .
./sms-code-webhook.exe
```

Windows 本地默认端口 `8080`，数据目录 `/data`（可设 `DATA_DIR=./data`）。

### 自行构建并推送到 Docker Hub

推送到 `main`/`master` 或打 `v*` 标签后，GitHub Actions 会构建并推送。

**仓库 Secrets（Settings → Secrets and variables → Actions）：**

| Secret | 说明 |
|--------|------|
| `DOCKERHUB_USERNAME` | Docker Hub 用户名（如 `ssssshql`） |
| `DOCKERHUB_TOKEN` | [Access Token](https://app.docker.com/accounts/ssssshql/settings/personal-access-tokens)（不要用登录密码） |

镜像名：

```
ssssshql/smsforwarder-webhook:latest
ssssshql/smsforwarder-webhook:v1.0.0
```

## 环境变量

| 变量 | 默认                          | 说明 |
|------|-----------------------------|------|
| `PORT` | `8080`（本地）/ `8080`（Compose） | 监听端口 |
| `DATA_DIR` | `/data`                     | 密钥文件目录 |
| `API_KEY` | 无                           | 手动指定密钥（最多建议 16 位）；不设则首次启动随机生成 16 位 hex 并写入 `$DATA_DIR/api_key` |
| `CODE_TTL` | `5m`                        | 验证码有效期，如 `10m`、`1h` |
| `HOST_PORT` | `8080`                      | Compose 宿主机映射端口 |

## 接口

### 健康检查

```
GET /health
```

无需鉴权。

### 1. 提交验证码（短信推送）

```
POST /api/v1/code
Content-Type: application/x-www-form-urlencoded
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `from` | 否 | 发送方号码 |
| `content` | 是 | JSON 字符串，见下方 |
| `timestamp` | 是 | 时间戳（秒或毫秒） |
| `sign` | 是 | 签名，见下方 |

**content 格式：**

```json
{
  "type": "疯狂水世界",
  "msg": "【广州益玩】您本次的验证码为809758，10分钟内有效……",
  "phone": "SIM1_152977314482"
}
```

- `type`：业务类型（业务标识，用于分槽；不同业务填不同 type）
- `msg`：短信正文（服务端从中提取验证码）
- `phone`：SIM 标识

#### SmsForwarder 短信转发规则模板

使用 [SmsForwarder](https://github.com/pppscn/SmsForwarder) 时，发送通道选 **Webhook**，请求方式 **POST**，URL 示例：

```
http://<服务器>:<端口>/api/v1/code
```

**`content` 模板：**

```json
{"type":"疯狂水世界","msg":"{{SMS}}","phone":"{{CARD_SLOT}}"}
```

说明：

| 占位符 | 含义 |
|--------|------|
| `{{SMS}}` | 短信全文 |
| `{{CARD_SLOT}}` | SIM 卡槽标识（如 `SIM1_152977314482`） |
| `type` | **固定写你的业务名**，不同业务建不同规则（如 `疯狂水世界`、`某某App`） |

签名：在 SmsForwarder Webhook 通道中开启签名，密钥填本服务的 `API_KEY`；算法与下文「签名规则」一致（`timestamp + "\n" + 密钥` → HmacSHA256 → Base64 → URL Encode）。

同一 `type` + `phone` 只保留最近一条验证码；提取时带上对应的 `type`、`phone` 即可。

**签名规则：**

```
签名字符串 = timestamp + "\n" + 密钥
sign = urlEncode( base64( HmacSHA256(签名字符串, 密钥) ) )
```

UTF-8；`sign` 传 urlencode 后或未 encode 的 base64 均可。

**示例：**

```bash
curl -X POST "http://localhost:8080/api/v1/code" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode 'from=46979788' \
  --data-urlencode 'timestamp=1784608529225' \
  --data-urlencode 'sign=xxxx' \
  --data-urlencode 'content={"type":"疯狂水世界","msg":"【广州益玩】您本次的验证码为809758","phone":"SIM1_152977314482"}'
```

成功响应：

```json
{
  "ok": true,
  "from": "46979788",
  "type": "疯狂水世界",
  "phone": "SIM1_152977314482",
  "code": "809758",
  "content": "【广州益玩】您本次的验证码为809758",
  "timestamp": 1784608529225,
  "created_at": "...",
  "expire_at": "..."
}
```

### 2. 提取验证码

```
GET /api/v1/code?key=<密钥>&type=...&phone=...&consume=true
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `key` | 是* | 密钥；也可用请求头 `X-API-Key` |
| `type` | 否 | 按业务类型过滤 |
| `phone` | 否 | 按 SIM 过滤 |
| `consume` | 否 | 默认 `true`（取后删除）；`false` 只读 |

过滤规则：

| 条件 | 行为 |
|------|------|
| `type` + `phone` | 精确匹配该槽 |
| 仅 `type` / 仅 `phone` | 在匹配项中取最新 |
| 都不传 | 全局最新一条 |

**示例：**

```bash
curl "http://localhost:8080/api/v1/code?key=YOUR_KEY&type=疯狂水世界&phone=SIM1_152977314482"
```

成功：

```json
{
  "ok": true,
  "type": "疯狂水世界",
  "phone": "SIM1_152977314482",
  "code": "809758",
  "consumed": true,
  ...
}
```

无验证码时仍返回 HTTP 200：

```json
{
  "ok": false,
  "error": "验证码不存在或已过期"
}
```

## 存储说明

- 仅内存，不落盘；容器/进程重启后清空
- 分槽 key：`type` + `phone`，同槽覆盖旧值
- 定时清理过期条目

## License

Private
