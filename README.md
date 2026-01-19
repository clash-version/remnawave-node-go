
# Remnawave Node Go

将 [remnawave/node](https://github.com/remnawave/node) (TypeScript/NestJS) 迁移到 Go 语言的项目。

## 快速开始

### 一键安装 (推荐)

```bash
# 使用 curl
curl -fsSL https://raw.githubusercontent.com/clash-version/remnawave-node-go/main/install.sh | bash

# 或使用 wget
wget -qO- https://raw.githubusercontent.com/clash-version/remnawave-node-go/main/install.sh | bash
```

安装完成后，配置节点：

```bash
# 设置节点 payload（从 Remnawave 面板获取）
remnawave-node-config set-payload <your-payload>

# 设置 SSL 证书
remnawave-node-config set-cert /path/to/cert.pem /path/to/key.pem

# 启动服务
systemctl start remnawave-node

# 设置开机自启
systemctl enable remnawave-node
```

### 更新

```bash
curl -fsSL https://raw.githubusercontent.com/clash-version/remnawave-node-go/main/install.sh | bash -s -- update
```

### 卸载

```bash
curl -fsSL https://raw.githubusercontent.com/clash-version/remnawave-node-go/main/install.sh | bash -s -- uninstall
```

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/clash-version/remnawave-node-go.git
cd remnawave-node-go

# 构建
make build

# 运行
./build/remnawave-node
```

### Docker

```bash
docker run -d \
  -e REMNAWAVE_NODE_PAYLOAD="your_base64_payload" \
  -v /path/to/cert.pem:/etc/ssl/cert.pem \
  -v /path/to/key.pem:/etc/ssl/key.pem \
  -p 443:443 \
  ghcr.io/clash-version/remnawave-node-go:latest
```

## 常用命令

```bash
# 查看服务状态
systemctl status remnawave-node

# 查看日志
journalctl -u remnawave-node -f

# 重启服务
systemctl restart remnawave-node

# 配置助手
remnawave-node-config --help
```

## 项目结构

```
.
├── cmd/node/           # 程序入口
├── internal/
│   ├── config/         # 配置管理
│   ├── middleware/     # HTTP 中间件 (JWT, 日志)
│   ├── server/         # HTTP 服务器和路由
│   └── services/       # 业务逻辑服务
├── pkg/
│   ├── crypto/         # SECRET_KEY 解析
│   ├── hashedset/      # 配置变更检测
│   ├── logger/         # Zap 日志
│   ├── supervisord/    # Supervisord XML-RPC 客户端
│   └── xtls/           # Xray gRPC 客户端
├── scripts/            # 安装脚本
├── deploy/             # 部署配置
└── .github/workflows/  # CI/CD
```

## 环境变量

| 变量名 | 必需 | 默认值 | 说明 |
|--------|------|--------|------|
| `SECRET_KEY` | ✅ | - | Base64 编码的 JSON，包含证书和密钥 |
| `NODE_PORT` | ❌ | 3000 | 主服务器端口 |
| `XTLS_IP` | ❌ | 127.0.0.1 | Xray gRPC 地址 |
| `XTLS_PORT` | ❌ | 61000 | Xray gRPC 端口 |
| `DISABLE_HASHED_SET_CHECK` | ❌ | false | 禁用配置变更检测 |

## 学习资料

- Go SDK: https://github.com/Jolymmiles/remnawave-api-go
- Xray-core 源码: https://github.com/XTLS/Xray-core

---

## 迁移计划

### 一、项目概述

**原项目技术栈：**
- TypeScript + NestJS (Node.js 框架)
- JWT 认证
- gRPC 与 Xray-core 通信 (通过 @remnawave/xtls-sdk)
- Supervisord 进程管理
- HTTPS 双向认证 (mTLS)

**目标技术栈：**
- Go 1.21+
- Gin/Fiber/Echo (HTTP 框架，推荐 Gin)
- gRPC 客户端连接 Xray-core
- JWT-go 认证
- Supervisord XML-RPC 客户端

---

### 二、核心模块分析

#### 1. Xray 模块 (`xray-core/`)
**功能：**
- 启动/停止 Xray 进程 (通过 Supervisord)
- 获取 Xray 状态和版本
- 节点健康检查
- 配置管理和热更新检测

**API 端点：**
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/node/xray/start` | 启动 Xray |
| POST | `/node/xray/stop` | 停止 Xray |
| GET | `/node/xray/status` | 获取状态 |
| GET | `/node/xray/node-health-check` | 健康检查 |

#### 2. Handler 模块 (`handler/`)
**功能：**
- 用户管理 (添加/删除用户)
- 支持多协议: VLESS, Trojan, Shadowsocks
- 批量用户操作

**API 端点：**
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/node/handler/add-user` | 添加单个用户 |
| POST | `/node/handler/add-users` | 批量添加用户 |
| POST | `/node/handler/remove-user` | 删除单个用户 |
| POST | `/node/handler/remove-users` | 批量删除用户 |
| GET | `/node/handler/get-inbound-users-count` | 获取入站用户数 |
| GET | `/node/handler/get-inbound-users` | 获取入站用户列表 |

#### 3. Stats 模块 (`stats/`)
**功能：**
- 用户流量统计
- 系统统计信息
- 入站/出站统计
- 用户在线状态

**API 端点：**
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/node/stats/get-user-online-status` | 用户在线状态 |
| GET | `/node/stats/get-users-stats` | 用户统计 |
| GET | `/node/stats/get-system-stats` | 系统统计 |
| GET | `/node/stats/get-inbound-stats` | 入站统计 |
| GET | `/node/stats/get-outbound-stats` | 出站统计 |
| GET | `/node/stats/get-all-inbounds-stats` | 所有入站统计 |
| GET | `/node/stats/get-all-outbounds-stats` | 所有出站统计 |
| GET | `/node/stats/get-combined-stats` | 综合统计 |

#### 4. Vision 模块 (`vision/`)
**功能：**
- IP 封禁/解封
- 动态路由规则管理

**API 端点：**
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/vision/block-ip` | 封禁 IP |
| POST | `/vision/unblock-ip` | 解封 IP |

#### 5. Internal 模块 (`internal/`)
**功能：**
- Xray 配置管理
- 用户 Hash 集合管理
- 配置变更检测

**API 端点 (内部端口 61001)：**
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/internal/get-config` | 获取 Xray 配置 |

---

### 三、关键技术细节

#### 3.1 SECRET_KEY 结构
SECRET_KEY 是 Base64 编码的 JSON，包含以下字段：
```json
{
    "caCertPem": "-----BEGIN CERTIFICATE-----...",
    "nodeCertPem": "-----BEGIN CERTIFICATE-----...",
    "nodeKeyPem": "-----BEGIN RSA PRIVATE KEY-----...",
    "jwtPublicKey": "-----BEGIN PUBLIC KEY-----..."
}
```

#### 3.2 网络架构
```
┌─────────────────────────────────────────────────────────────────┐
│                        Remnawave Node                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────┐     ┌──────────────────┐                 │
│  │   Main Server    │     │  Internal Server │                 │
│  │   (mTLS + JWT)   │     │   (Port 61001)   │                 │
│  │   Port: $NODE_PORT│     │   127.0.0.1 only │                 │
│  └────────┬─────────┘     └────────┬─────────┘                 │
│           │                        │                            │
│           │  ┌─────────────────────┘                            │
│           │  │                                                  │
│           ▼  ▼                                                  │
│  ┌──────────────────┐     ┌──────────────────┐                 │
│  │   Xray gRPC      │     │   Supervisord    │                 │
│  │   127.0.0.1:61000│     │   127.0.0.1:61002│                 │
│  └──────────────────┘     └──────────────────┘                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

#### 3.3 端口配置
| 端口 | 用途 | 监听地址 |
|------|------|----------|
| `$NODE_PORT` | 主 API (mTLS) | 0.0.0.0 |
| 61000 | Xray gRPC API | 127.0.0.1 |
| 61001 | Internal API | 127.0.0.1 |
| 61002 | Supervisord | 127.0.0.1 |

#### 3.4 Xray 默认配置注入
启动时会自动注入以下配置：
```json
{
    "api": {
        "services": ["HandlerService", "StatsService", "RoutingService"],
        "listen": "127.0.0.1:61000",
        "tag": "REMNAWAVE_API"
    },
    "stats": {},
    "policy": {
        "levels": {
            "0": {
                "statsUserUplink": true,
                "statsUserDownlink": true,
                "statsUserOnline": false
            }
        },
        "system": {
            "statsInboundDownlink": true,
            "statsInboundUplink": true,
            "statsOutboundDownlink": true,
            "statsOutboundUplink": true
        }
    }
}
```

#### 3.5 JWT 认证
- 算法: RS256
- 公钥来源: SECRET_KEY 中的 `jwtPublicKey`
- Token 位置: `Authorization: Bearer <token>`

#### 3.6 mTLS 双向认证
- CA 证书: `caCertPem` (验证客户端证书)
- 服务器证书: `nodeCertPem`
- 服务器私钥: `nodeKeyPem`
- 配置: `requestCert: true, rejectUnauthorized: true`

#### 3.7 用户协议类型
| 类型 | 字段 |
|------|------|
| VLESS | `uuid`, `flow` (xtls-rprx-vision 或空) |
| Trojan | `password` |
| Shadowsocks | `password`, `cipherType`, `ivCheck` |

**Shadowsocks 加密类型 (CipherType):**
```go
const (
    AES_128_GCM       = 5
    AES_256_GCM       = 6
    CHACHA20_POLY1305 = 7
    XCHACHA20_POLY1305 = 8
    NONE              = 9
)
```

#### 3.8 Supervisord 配置
```ini
[inet_http_server]
port=127.0.0.1:61002
username=remnawave
password=glcmYQLRwPXDXIBq

[program:xray]
command=/usr/local/bin/xray run -config /opt/remnawave-node/config/xray.json
autostart=false
autorestart=true
```

#### 3.9 配置变更检测 (HashedSet)
使用 Hash 集合检测用户配置是否变化，避免不必要的 Xray 重启：
- `emptyConfigHash`: 空配置的 Hash 值
- `inbounds[].hash`: 每个入站的用户集合 Hash
- `inbounds[].usersCount`: 用户数量

---

### 四、Go 项目结构设计

```
remnawave-node-go/
├── cmd/
│   └── node/
│       └── main.go                 # 程序入口
├── internal/
│   ├── config/
│   │   ├── config.go               # 配置结构
│   │   └── loader.go               # 配置加载
│   ├── middleware/
│   │   ├── auth.go                 # JWT 认证中间件
│   │   ├── logger.go               # 日志中间件
│   │   └── recovery.go             # 错误恢复中间件
│   ├── handler/
│   │   ├── handler.go              # Handler 控制器
│   │   ├── service.go              # Handler 业务逻辑
│   │   └── dto.go                  # 数据传输对象
│   ├── stats/
│   │   ├── handler.go              # Stats 控制器
│   │   ├── service.go              # Stats 业务逻辑
│   │   └── dto.go
│   ├── xray/
│   │   ├── handler.go              # Xray 控制器
│   │   ├── service.go              # Xray 业务逻辑
│   │   └── dto.go
│   ├── vision/
│   │   ├── handler.go              # Vision 控制器
│   │   ├── service.go              # Vision 业务逻辑
│   │   └── dto.go
│   ├── internal/
│   │   └── service.go              # Internal 服务 (配置管理)
│   └── server/
│       ├── server.go               # HTTP 服务器
│       └── routes.go               # 路由注册
├── pkg/
│   ├── xtls/
│   │   ├── client.go               # Xray gRPC 客户端
│   │   ├── handler.go              # 用户操作
│   │   ├── stats.go                # 统计接口
│   │   └── router.go               # 路由操作
│   ├── supervisord/
│   │   └── client.go               # Supervisord XML-RPC 客户端
│   ├── crypto/
│   │   └── payload.go              # SECRET_KEY 解析
│   └── hashedset/
│       └── hashedset.go            # Hash 集合实现
├── api/
│   └── proto/                      # Xray gRPC proto 文件
├── scripts/
│   ├── install.sh                  # 一键安装脚本
│   ├── uninstall.sh                # 卸载脚本
│   └── update.sh                   # 更新脚本
├── deployments/
│   ├── remnawave-node.service      # systemd 服务文件
│   └── supervisord.conf            # supervisord 配置
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

### 四、迁移阶段计划

#### 阶段 1: 基础框架搭建 (预计 2-3 天)

**任务清单：**
- [ ] 初始化 Go 模块 (`go mod init`)
- [ ] 搭建 HTTP 服务器框架 (Gin)
- [ ] 实现配置加载 (支持环境变量)
- [ ] 实现 SECRET_KEY 解析 (Base64 + JSON)
- [ ] 实现 mTLS 配置
- [ ] 实现 JWT 认证中间件
- [ ] 实现日志系统

**关键配置项：**
```go
type Config struct {
    NodePort             int    `env:"NODE_PORT" default:"3000"`
    SecretKey            string `env:"SECRET_KEY" required:"true"`
    XtlsIP               string `env:"XTLS_IP" default:"127.0.0.1"`
    XtlsPort             string `env:"XTLS_PORT" default:"61000"`
    DisableHashedSetCheck bool  `env:"DISABLE_HASHED_SET_CHECK" default:"false"`
}
```

#### 阶段 2: Xray gRPC 客户端 (预计 3-4 天)

**任务清单：**
- [ ] 导入/生成 Xray gRPC proto 文件
- [ ] 实现 gRPC 客户端连接池
- [ ] 实现 Handler API (添加/删除用户)
- [ ] 实现 Stats API (统计查询)
- [ ] 实现 Router API (路由规则)
- [ ] 编写单元测试

**参考资源：**
- Xray-core gRPC 定义: `github.com/xtls/xray-core/app/proxyman/command`
- Go SDK 参考: `github.com/Jolymmiles/remnawave-api-go`

#### 阶段 3: Supervisord 客户端 (预计 1-2 天)

**任务清单：**
- [ ] 实现 XML-RPC 客户端
- [ ] 实现进程管理接口 (start/stop/restart/status)
- [ ] 实现进程信息查询

**核心方法：**
```go
type SupervisordClient interface {
    StartProcess(name string) error
    StopProcess(name string) error
    RestartProcess(name string) error
    GetProcessInfo(name string) (*ProcessInfo, error)
    GetState() (*State, error)
}
```

#### 阶段 4: 业务模块实现 (预计 5-7 天)

**4.1 Xray 模块**
- [ ] 实现 StartXray 逻辑 (配置解析 + 进程管理 + 健康检查)
- [ ] 实现 StopXray 逻辑
- [ ] 实现状态查询和版本获取
- [ ] 实现节点健康检查

**4.2 Handler 模块**
- [ ] 实现用户添加 (VLESS/Trojan/Shadowsocks)
- [ ] 实现用户删除
- [ ] 实现批量操作
- [ ] 实现入站用户查询

**4.3 Stats 模块**
- [ ] 实现用户统计查询
- [ ] 实现系统统计查询
- [ ] 实现入站/出站统计
- [ ] 实现用户在线状态查询

**4.4 Vision 模块**
- [ ] 实现 IP 封禁
- [ ] 实现 IP 解封

**4.5 Internal 模块**
- [ ] 实现配置存储和管理
- [ ] 实现 HashedSet (用户 Hash 集合)
- [ ] 实现配置变更检测

#### 阶段 5: 测试与优化 (预计 3-4 天)

**任务清单：**
- [ ] 编写集成测试
- [ ] 性能测试和优化
- [ ] 内存泄漏检查
- [ ] 并发安全检查
- [ ] API 兼容性测试

#### 阶段 6: 部署准备 (预计 2-3 天)

**任务清单：**
- [ ] 编写一键安装脚本 `install.sh`
- [ ] 编写 systemd 服务文件
- [ ] 编写 supervisord.conf (管理 xray 进程)
- [ ] 配置 GitHub Releases 自动发布
- [ ] 创建 GitHub Actions CI/CD (多平台编译)
- [ ] 编写卸载脚本 `uninstall.sh`

**安装方式：**
```bash
# 一键安装
bash <(curl -Ls https://raw.githubusercontent.com/your-repo/remnawave-node-go/main/install.sh) \
  -h https://api.remnawave.com \
  -t YOUR_NODE_TOKEN

# 或下载后安装
curl -O https://raw.githubusercontent.com/your-repo/remnawave-node-go/main/install.sh
chmod +x install.sh
./install.sh -h https://api.remnawave.com -t YOUR_NODE_TOKEN
```

**安装脚本功能：**
- 自动检测系统架构 (amd64/arm64)
- 自动下载对应版本二进制文件
- 配置 systemd 服务
- 安装 Xray-core
- 配置 supervisord
- 自动启动服务

---

### 五、核心代码示例

#### 5.1 配置解析 (SECRET_KEY)

```go
package crypto

import (
    "encoding/base64"
    "encoding/json"
)

type NodePayload struct {
    CACertPem    string `json:"caCertPem"`
    NodeCertPem  string `json:"nodeCertPem"`
    NodeKeyPem   string `json:"nodeKeyPem"`
    JWTPublicKey string `json:"jwtPublicKey"`
}

func ParseNodePayload(secretKey string) (*NodePayload, error) {
    decoded, err := base64.StdEncoding.DecodeString(secretKey)
    if err != nil {
        return nil, err
    }
    
    var payload NodePayload
    if err := json.Unmarshal(decoded, &payload); err != nil {
        return nil, err
    }
    
    return &payload, nil
}
```

#### 5.2 gRPC 客户端示例

```go
package xtls

import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    
    handlerService "github.com/xtls/xray-core/app/proxyman/command"
    statsService "github.com/xtls/xray-core/app/stats/command"
)

type XtlsClient struct {
    conn    *grpc.ClientConn
    handler handlerService.HandlerServiceClient
    stats   statsService.StatsServiceClient
}

func NewXtlsClient(addr string) (*XtlsClient, error) {
    conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return nil, err
    }
    
    return &XtlsClient{
        conn:    conn,
        handler: handlerService.NewHandlerServiceClient(conn),
        stats:   statsService.NewStatsServiceClient(conn),
    }, nil
}
```

#### 5.3 JWT 中间件示例

```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
)

func JWTAuth(publicKey string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenString := c.GetHeader("Authorization")
        // ... 验证逻辑
        c.Next()
    }
}
```

---

### 六、依赖库推荐

| 功能 | 推荐库 | 说明 |
|------|--------|------|
| HTTP 框架 | `github.com/gin-gonic/gin` | 高性能 HTTP 框架 |
| 配置管理 | `github.com/spf13/viper` | 支持多种配置格式 |
| 日志 | `go.uber.org/zap` | 高性能结构化日志 |
| JWT | `github.com/golang-jwt/jwt/v5` | JWT 认证 |
| gRPC | `google.golang.org/grpc` | gRPC 客户端 |
| 验证 | `github.com/go-playground/validator` | 请求参数验证 |
| 并发控制 | `golang.org/x/sync/semaphore` | 信号量实现 |

---

### 七、风险与注意事项

1. **gRPC Proto 兼容性**: 确保使用与 Xray-core 版本匹配的 proto 文件
2. **并发安全**: Go 的 map 需要使用 sync.Map 或 mutex 保护
3. **内存管理**: 注意大配置文件的内存占用
4. **错误处理**: Go 的错误处理与 TypeScript 不同，需要显式处理
5. **API 兼容性**: 确保请求/响应格式与原项目完全一致

---

### 八、预估时间线

| 阶段 | 预计时间 | 里程碑 |
|------|----------|--------|
| 阶段 1: 基础框架 | 2-3 天 | HTTP 服务器 + 认证 |
| 阶段 2: gRPC 客户端 | 3-4 天 | Xray 通信完成 |
| 阶段 3: Supervisord | 1-2 天 | 进程管理完成 |
| 阶段 4: 业务模块 | 5-7 天 | 核心功能完成 |
| 阶段 5: 测试优化 | 3-4 天 | 测试覆盖 |
| 阶段 6: 部署脚本 | 2-3 天 | 一键安装就绪 |
| **总计** | **16-24 天** | |

---

### 九、安装脚本设计

#### 9.1 安装脚本参数

```bash
./install.sh [选项]

选项:
  -h, --host <URL>        Remnawave API 地址 (必需)
  -t, --token <TOKEN>     节点 Token (必需)
  -p, --port <PORT>       节点端口 (默认: 3000)
  -v, --version <VER>     指定安装版本 (默认: latest)
  --xray-version <VER>    指定 Xray 版本 (默认: latest)
  --uninstall             卸载节点
  --update                更新节点
  --help                  显示帮助信息
```

#### 9.2 安装目录结构

```
/opt/remnawave-node/
├── bin/
│   ├── remnawave-node          # 主程序
│   └── xray                    # Xray-core 二进制
├── config/
│   ├── config.env              # 环境变量配置
│   └── xray.json               # Xray 配置 (运行时生成)
├── logs/
│   ├── node.log                # 节点日志
│   └── xray.log                # Xray 日志
└── supervisord/
    ├── supervisord.conf        # supervisord 主配置
    └── supervisord.sock        # supervisord socket
```

#### 9.3 安装脚本核心逻辑

```bash
#!/bin/bash

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

# 默认值
INSTALL_DIR="/opt/remnawave-node"
NODE_PORT=3000
VERSION="latest"
XRAY_VERSION="latest"
GITHUB_REPO="your-org/remnawave-node-go"

# 检测系统架构
detect_arch() {
    case "$(uname -m)" in
        x86_64)  echo "amd64" ;;
        aarch64) echo "arm64" ;;
        armv7l)  echo "arm" ;;
        *)       echo "unsupported" ;;
    esac
}

# 检测操作系统
detect_os() {
    case "$(uname -s)" in
        Linux)  echo "linux" ;;
        Darwin) echo "darwin" ;;
        *)      echo "unsupported" ;;
    esac
}

# 下载文件
download_file() {
    local url=$1
    local dest=$2
    if command -v curl &> /dev/null; then
        curl -fsSL "$url" -o "$dest"
    elif command -v wget &> /dev/null; then
        wget -q "$url" -O "$dest"
    else
        echo -e "${RED}错误: 需要 curl 或 wget${NC}"
        exit 1
    fi
}

# 安装主程序
install_node() {
    local os=$(detect_os)
    local arch=$(detect_arch)
    
    echo -e "${GREEN}检测到系统: ${os}/${arch}${NC}"
    
    # 创建目录
    mkdir -p "$INSTALL_DIR"/{bin,config,logs,supervisord}
    
    # 下载二进制文件
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/remnawave-node-${os}-${arch}"
    echo -e "${YELLOW}下载 remnawave-node...${NC}"
    download_file "$download_url" "$INSTALL_DIR/bin/remnawave-node"
    chmod +x "$INSTALL_DIR/bin/remnawave-node"
    
    # 安装 Xray
    install_xray
    
    # 生成配置
    generate_config
    
    # 安装 supervisord (如果需要)
    install_supervisord
    
    # 配置 systemd
    setup_systemd
    
    # 启动服务
    systemctl daemon-reload
    systemctl enable remnawave-node
    systemctl start remnawave-node
    
    echo -e "${GREEN}安装完成!${NC}"
}

# 安装 Xray-core
install_xray() {
    echo -e "${YELLOW}安装 Xray-core...${NC}"
    bash <(curl -Ls https://raw.githubusercontent.com/XTLS/Xray-install/main/install-release.sh) install
    ln -sf /usr/local/bin/xray "$INSTALL_DIR/bin/xray"
}

# 生成配置文件
generate_config() {
    cat > "$INSTALL_DIR/config/config.env" << EOF
NODE_PORT=${NODE_PORT}
SECRET_KEY=${NODE_TOKEN}
XTLS_IP=127.0.0.1
XTLS_PORT=61000
DISABLE_HASHED_SET_CHECK=false
EOF
}

# 配置 systemd
setup_systemd() {
    cat > /etc/systemd/system/remnawave-node.service << EOF
[Unit]
Description=Remnawave Node
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=${INSTALL_DIR}/config/config.env
ExecStart=${INSTALL_DIR}/bin/remnawave-node
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
}

# 主函数
main() {
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--host)    API_HOST="$2"; shift 2 ;;
            -t|--token)   NODE_TOKEN="$2"; shift 2 ;;
            -p|--port)    NODE_PORT="$2"; shift 2 ;;
            -v|--version) VERSION="$2"; shift 2 ;;
            --uninstall)  uninstall; exit 0 ;;
            --update)     update; exit 0 ;;
            --help)       show_help; exit 0 ;;
            *)            echo "未知参数: $1"; exit 1 ;;
        esac
    done
    
    # 验证必需参数
    if [[ -z "$API_HOST" || -z "$NODE_TOKEN" ]]; then
        echo -e "${RED}错误: 必须提供 -h 和 -t 参数${NC}"
        show_help
        exit 1
    fi
    
    install_node
}

main "$@"
```

#### 9.4 systemd 服务文件

```ini
# /etc/systemd/system/remnawave-node.service
[Unit]
Description=Remnawave Node - Xray Proxy Management Node
Documentation=https://github.com/your-org/remnawave-node-go
After=network.target nss-lookup.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/remnawave-node
EnvironmentFile=/opt/remnawave-node/config/config.env
ExecStart=/opt/remnawave-node/bin/remnawave-node
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
LimitNOFILE=65535

# 安全配置
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/remnawave-node

[Install]
WantedBy=multi-user.target
```

#### 9.5 常用命令

```bash
# 查看服务状态
systemctl status remnawave-node

# 查看日志
journalctl -u remnawave-node -f

# 重启服务
systemctl restart remnawave-node

# 停止服务
systemctl stop remnawave-node

# 更新节点
bash <(curl -Ls https://raw.githubusercontent.com/your-repo/remnawave-node-go/main/install.sh) --update

# 卸载节点
bash <(curl -Ls https://raw.githubusercontent.com/your-repo/remnawave-node-go/main/install.sh) --uninstall
```

---

### 十、下一步行动

1. 确认 Go 版本和开发环境
2. 初始化项目结构
3. 从阶段 1 开始实施
4. 每个阶段完成后进行代码审查

---

### 十一、GitHub Actions CI/CD

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        goos: [linux, darwin]
        goarch: [amd64, arm64]
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
        run: |
          go build -ldflags="-s -w" -o remnawave-node-${{ matrix.goos }}-${{ matrix.goarch }} ./cmd/node
      
      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: remnawave-node-${{ matrix.goos }}-${{ matrix.goarch }}
          path: remnawave-node-${{ matrix.goos }}-${{ matrix.goarch }}

  release:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Download all artifacts
        uses: actions/download-artifact@v4
      
      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            remnawave-node-*/remnawave-node-*
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

### 十二、API 请求/响应格式

#### 12.1 Xray 模块

**POST /node/xray/start**
```json
// Request
{
    "internals": {
        "forceRestart": false,
        "hashes": {
            "emptyConfig": "abc123...",
            "inbounds": [
                { "tag": "vless-in", "hash": "def456...", "usersCount": 100 }
            ]
        }
    },
    "xrayConfig": { /* Xray 完整配置 */ }
}

// Response
{
    "response": {
        "isStarted": true,
        "version": "1.8.24",
        "error": null,
        "systemInformation": {
            "cpuCores": 4,
            "cpuModel": "Intel(R) Xeon(R)/2.5 GHz",
            "memoryTotal": "8 GB"
        },
        "nodeInformation": {
            "version": "2.5.0"
        }
    }
}
```

**GET /node/xray/stop**
```json
// Response
{
    "response": {
        "isStopped": true
    }
}
```

**GET /node/xray/status**
```json
// Response
{
    "response": {
        "isRunning": true,
        "version": "1.8.24"
    }
}
```

**GET /node/xray/node-health-check**
```json
// Response
{
    "response": {
        "isHealthy": true,
        "isXrayRunning": true,
        "xrayVersion": "1.8.24",
        "nodeVersion": "2.5.0"
    }
}
```

#### 12.2 Handler 模块

**POST /node/handler/add-user**
```json
// Request
{
    "data": [
        {
            "type": "vless",
            "tag": "vless-in",
            "username": "user@example.com",
            "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
            "flow": "xtls-rprx-vision"
        }
    ],
    "hashData": {
        "vlessUuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
        "prevVlessUuid": null
    }
}

// Response
{
    "response": {
        "success": true,
        "error": null
    }
}
```

**POST /node/handler/add-users (批量)**
```json
// Request
{
    "affectedInboundTags": ["vless-in", "trojan-in"],
    "users": [
        {
            "inboundData": [
                { "type": "vless", "tag": "vless-in", "flow": "xtls-rprx-vision" },
                { "type": "trojan", "tag": "trojan-in" }
            ],
            "userData": {
                "userId": "user@example.com",
                "hashUuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
                "vlessUuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
                "trojanPassword": "password123",
                "ssPassword": "sspassword123"
            }
        }
    ]
}

// Response
{
    "response": {
        "success": true,
        "error": null
    }
}
```

**POST /node/handler/remove-user**
```json
// Request
{
    "username": "user@example.com",
    "hashData": {
        "vlessUuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    }
}

// Response
{
    "response": {
        "success": true,
        "error": null
    }
}
```

**POST /node/handler/remove-users (批量)**
```json
// Request
{
    "users": [
        { "userId": "user1@example.com", "hashUuid": "uuid1" },
        { "userId": "user2@example.com", "hashUuid": "uuid2" }
    ]
}

// Response
{
    "response": {
        "success": true,
        "error": null
    }
}
```

#### 12.3 Stats 模块

**POST /node/stats/get-user-online-status**
```json
// Request
{ "username": "user@example.com" }

// Response
{ "response": { "online": true } }
```

**POST /node/stats/get-users-stats**
```json
// Request
{ "reset": true }

// Response
{
    "response": {
        "users": [
            { "username": "user1", "uplink": 1024000, "downlink": 2048000 },
            { "username": "user2", "uplink": 512000, "downlink": 1024000 }
        ]
    }
}
```

**GET /node/stats/get-system-stats**
```json
// Response
{
    "response": {
        "numGoroutine": 50,
        "numGC": 100,
        "alloc": 10485760,
        "totalAlloc": 104857600,
        "sys": 52428800,
        "mallocs": 500000,
        "frees": 450000,
        "liveObjects": 50000,
        "uptime": 86400
    }
}
```

**POST /node/stats/get-inbound-stats**
```json
// Request
{ "tag": "vless-in", "reset": false }

// Response
{
    "response": {
        "inbound": "vless-in",
        "uplink": 10240000,
        "downlink": 20480000
    }
}
```

**POST /node/stats/get-combined-stats**
```json
// Request
{ "reset": false }

// Response
{
    "response": {
        "inbounds": [
            { "inbound": "vless-in", "uplink": 1024, "downlink": 2048 }
        ],
        "outbounds": [
            { "outbound": "direct", "uplink": 1024, "downlink": 2048 }
        ]
    }
}
```

#### 12.4 Vision 模块 (内部端口 61001)

**POST /vision/block-ip**
```json
// Request
{ "ip": "192.168.1.100" }

// Response
{
    "response": {
        "success": true,
        "error": null
    }
}
```

**POST /vision/unblock-ip**
```json
// Request
{ "ip": "192.168.1.100" }

// Response
{
    "response": {
        "success": true,
        "error": null
    }
}
```

---

### 十三、错误码定义

| 错误码 | 说明 |
|--------|------|
| `INTERNAL_SERVER_ERROR` | 内部服务器错误 |
| `FAILED_TO_GET_SYSTEM_STATS` | 获取系统统计失败 |
| `FAILED_TO_GET_USERS_STATS` | 获取用户统计失败 |
| `FAILED_TO_GET_INBOUND_STATS` | 获取入站统计失败 |
| `FAILED_TO_GET_OUTBOUND_STATS` | 获取出站统计失败 |
| `FAILED_TO_GET_INBOUNDS_STATS` | 获取所有入站统计失败 |
| `FAILED_TO_GET_OUTBOUNDS_STATS` | 获取所有出站统计失败 |
| `FAILED_TO_GET_COMBINED_STATS` | 获取综合统计失败 |
| `FAILED_TO_GET_INBOUND_USERS` | 获取入站用户失败 |