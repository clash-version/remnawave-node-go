# Remnawave Node Go - 配置指南

## 环境变量说明

### 必需配置

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `REMNAWAVE_NODE_PAYLOAD` | 节点 Payload，从 Remnawave 面板获取 (Base64 编码) | `eyJhcGlVcmwiOi...` |
| `SSL_CERT_PATH` | SSL 证书路径 | `/etc/remnawave-node/cert.pem` |
| `SSL_KEY_PATH` | SSL 私钥路径 | `/etc/remnawave-node/key.pem` |

### 可选配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `API_PORT` | 从 Payload 读取 | API 服务端口 |
| `XRAY_API_PORT` | `61000` | Xray gRPC API 端口 |
| `INTERNAL_API_PORT` | `61001` | 内部 API 端口 (Vision IP 封禁) |
| `SUPERVISORD_RPC_PORT` | `61010` | Supervisord RPC 端口 |
| `LOG_LEVEL` | `info` | 日志级别: debug, info, warn, error |
| `DISABLE_HASHED_SET_CHECK` | `false` | 禁用配置哈希比较 |

---

## remnawave-node-config 配置助手

`remnawave-node-config` 是安装脚本自动创建的配置管理工具，简化节点配置流程。

### 命令列表

```bash
# 查看帮助
remnawave-node-config --help

# 设置节点 Payload（从 Remnawave 面板复制）
remnawave-node-config set-payload <your-base64-payload>

# 设置 SSL 证书路径
remnawave-node-config set-cert /path/to/cert.pem /path/to/key.pem

# 查看当前配置
remnawave-node-config show

# 编辑配置文件（使用默认编辑器）
remnawave-node-config edit

# 查看服务状态
remnawave-node-config status

# 查看实时日志
remnawave-node-config logs

# 重启服务
remnawave-node-config restart
```

### 使用示例

#### 1. 首次配置

```bash
# 从 Remnawave 面板复制 Payload 并设置
remnawave-node-config set-payload eyJhcGlVcmwiOiJodHRwczovL2FwaS5leGFtcGxlLmNvbSIsImp3dFB1YmxpY0tleSI6Ii0tLS0tQkVHSU4gUFVCTElDIEtFWS0tLS0tLi4uIn0=

# 设置 SSL 证书（使用 Let's Encrypt 或自签名证书）
remnawave-node-config set-cert /etc/letsencrypt/live/node.example.com/fullchain.pem /etc/letsencrypt/live/node.example.com/privkey.pem

# 启动服务
systemctl start remnawave-node
systemctl enable remnawave-node
```

#### 2. 查看当前配置

```bash
$ remnawave-node-config show

Current configuration (/etc/remnawave-node/env):
----------------------------------------
REMNAWAVE_NODE_PAYLOAD=eyJhcGlVcmwiOi...
SSL_CERT_PATH=/etc/letsencrypt/live/node.example.com/fullchain.pem
SSL_KEY_PATH=/etc/letsencrypt/live/node.example.com/privkey.pem
```

#### 3. 调试问题

```bash
# 查看服务状态
remnawave-node-config status

# 实时查看日志
remnawave-node-config logs

# 重启服务
remnawave-node-config restart
```

---

## 配置文件位置

| 文件 | 路径 | 说明 |
|------|------|------|
| 主程序 | `/usr/local/bin/remnawave-node` | 节点二进制文件 |
| 配置助手 | `/usr/local/bin/remnawave-node-config` | 配置管理工具 |
| 环境配置 | `/etc/remnawave-node/env` | 环境变量配置文件 |
| 数据目录 | `/var/lib/remnawave-node/` | Xray 配置和数据 |
| 服务文件 | `/etc/systemd/system/remnawave-node.service` | systemd 服务 |

---

## 获取 Node Payload

1. 登录 Remnawave 面板
2. 进入 **节点管理** > **添加节点**
3. 选择 **Node Go** 类型
4. 复制生成的 Payload（Base64 字符串）
5. 使用 `remnawave-node-config set-payload <payload>` 设置

Payload 包含的信息：
- API 地址
- JWT 公钥（用于验证请求）
- 其他节点配置

---

## SSL 证书配置

### 使用 Let's Encrypt (推荐)

```bash
# 安装 certbot
apt install certbot

# 获取证书
certbot certonly --standalone -d node.example.com

# 配置节点使用证书
remnawave-node-config set-cert \
  /etc/letsencrypt/live/node.example.com/fullchain.pem \
  /etc/letsencrypt/live/node.example.com/privkey.pem

# 重启服务
remnawave-node-config restart
```

### 使用自签名证书

```bash
# 生成自签名证书
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/remnawave-node/key.pem \
  -out /etc/remnawave-node/cert.pem \
  -subj "/CN=node.example.com"

# 配置节点
remnawave-node-config set-cert \
  /etc/remnawave-node/cert.pem \
  /etc/remnawave-node/key.pem
```

---

## 常见问题

### 服务启动失败

```bash
# 检查日志
journalctl -u remnawave-node -n 50

# 常见原因：
# 1. Payload 未设置或格式错误
# 2. SSL 证书路径错误或文件不存在
# 3. 端口被占用
```

### 无法连接到面板

```bash
# 检查网络连接
curl -v https://api.remnawave.com

# 检查防火墙
ufw status
iptables -L -n

# 确保 API_PORT (默认 443) 已开放
```

### 更新节点

```bash
# 使用安装脚本更新
curl -fsSL https://raw.githubusercontent.com/clash-version/remnawave-node-go/main/install.sh | bash -s -- update
```
