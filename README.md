# AlertEngine

AlertEngine 是一个独立的、可扩展的 Prometheus 告警规则引擎组件，用于实现灵活的告警管理和通知。

## 特性

- ✨ **独立组件**: 完全独立的告警引擎，可与多个 Prometheus 实例集成
- 🔄 **动态规则管理**: 通过 API 动态加载和更新告警规则，无需重启
- 📚 **规则历史管理**: 自动保存规则变更历史，支持版本追溯
- 📊 **完善的监控**: 内置 Prometheus 指标，实时监控告警引擎状态
- 🚀 **高性能**: 基于 Prometheus 官方规则引擎，稳定可靠
- 🔒 **安全认证**: 支持 Token 认证，保障数据安全
- 🐳 **容器化**: 提供 Docker 镜像和 docker-compose 配置
- 🛠️ **易于部署**: 支持二进制、Docker、Systemd 等多种部署方式

## 架构

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   用户界面   │─────▶│  Web网关      │◀─────│ Prometheus  │
└─────────────┘      └──────────────┘      └─────────────┘
                            │                      ▲
                            ▼                      │
                     ┌──────────────┐              │
                     │  AlertEngine │──────────────┘
                     └──────────────┘
                            │
                            ▼
                     ┌──────────────┐
                     │  通知渠道     │
                     └──────────────┘
```

### 核心组件

1. **Reloader**: 定期从网关同步规则和数据源配置
2. **Manager**: 每个 Prometheus 数据源对应一个管理器
3. **Storage**: 规则文件存储和版本管理
4. **Metrics**: 监控指标收集和暴露

## 快速开始

### 前置要求

- Go 1.21+
- Prometheus (可选，用于测试)
- 网关服务 (提供规则和数据源 API)

### 安装

#### 方式1: 从源码构建

```bash
# 克隆仓库
git clone https://github.com/will-yinchengxin/alertengine.git
cd alertengine

# 下载依赖
make deps

# 构建
make build

# 运行
./build/alertengine -config config.yml
```

#### 方式2: 使用 Docker

```bash
# 构建镜像
make docker

# 或使用 docker-compose
docker-compose up -d
```

#### 方式3: 系统服务安装

```bash
# 安装二进制文件和配置
sudo make install

# 创建用户
sudo useradd -r -s /bin/false alertengine

# 创建目录
sudo mkdir -p /var/lib/alertengine/rules /var/log/alertengine
sudo chown -R alertengine:alertengine /var/lib/alertengine /var/log/alertengine

# 安装 systemd 服务
sudo cp deploy/systemd/alertengine.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable alertengine
sudo systemctl start alertengine
```

## 配置

配置文件示例 (`config.yml`):

```yaml
# 告警通知重试次数
notify_retries: 3

# 网关配置
gateway:
  url: "http://localhost:32002"
  rule_path: "/api/v1/rules"
  prom_path: "/api/v1/proms"
  notify_path: "/api/v1/alerts"
  timeout: 10s

# 规则评估间隔
evaluation_interval: 30s

# 规则重载间隔
reload_interval: 5m

# API认证Token
auth_token: "your-secret-token"

# 规则存储配置
storage:
  rule_dir: "/var/lib/alertengine/rules"
  retention_days: 30
  enable_history: true

# 日志配置
log:
  level: "info"
  format: "json"
  output_path: "/var/log/alertengine/alertengine.log"

# 指标端口
metrics_port: 9090
```

### 配置说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `notify_retries` | 告警通知失败重试次数 | 3 |
| `gateway.url` | 网关服务地址 | http://localhost:32002 |
| `evaluation_interval` | 规则评估间隔 | 30s |
| `reload_interval` | 规则重载间隔 | 5m |
| `storage.rule_dir` | 规则文件存储目录 | /var/lib/alertengine/rules |
| `storage.retention_days` | 规则历史保留天数 | 30 |
| `storage.enable_history` | 是否启用历史版本 | true |

## 使用说明

### 规则格式

AlertEngine 使用与 Prometheus 兼容的规则格式。网关 API 返回的规则会被转换为标准的 Prometheus YAML 格式。

示例规则:

```json
{
  "id": 1,
  "prom_id": 1,
  "expr": "node_memory_Active_bytes{instance=\"172.16.27.76:9100\"}",
  "op": ">",
  "value": "0",
  "for": "120s",
  "labels": {},
  "summary": "内存告警",
  "description": "内存使用率过高"
}
```

转换后的 YAML:

```yaml
groups:
  - name: ruleengine
    rules:
      - alert: "1"
        expr: node_memory_Active_bytes{instance="172.16.27.76:9100"} > 0
        for: 120s
        labels: {}
        annotations:
          rule_id: "1"
          prom_id: "1"
          summary: "内存告警"
          description: "内存使用率过高"
```

### 规则历史查看

当启用历史记录时，规则文件会按以下结构存储:

```
/var/lib/alertengine/rules/
├── prom_1/
│   ├── current.yml              # 当前规则
│   └── history/
│       ├── rule_20260203_140000.yml
│       ├── rule_20260203_150000.yml
│       └── rule_20260203_160000.yml
└── prom_2/
    ├── current.yml
    └── history/
        └── ...
```

### 监控指标

AlertEngine 在 `:9090/metrics` 端点暴露以下指标:

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `alertengine_rules_loaded` | Gauge | 已加载的规则数量 |
| `alertengine_notifications_sent_total` | Counter | 发送的告警通知总数 |
| `alertengine_notify_errors_total` | Counter | 通知发送失败总数 |
| `alertengine_reload_success_total` | Counter | 规则重载成功次数 |
| `alertengine_reload_errors_total` | Counter | 规则重载失败次数 |
| `alertengine_evaluation_duration_seconds` | Histogram | 规则评估耗时 |
| `alertengine_active_managers` | Gauge | 活跃管理器数量 |

### 健康检查

- **健康检查**: `http://localhost:8080/health` - 服务是否运行
- **就绪检查**: `http://localhost:8080/ready` - 是否有活跃的管理器

## API 接口要求

AlertEngine 需要网关提供以下 API:

### 1. 获取规则列表

```
GET /api/v1/rules
Header: Token: <auth_token>

Response:
{
  "code": 0,
  "msg": "success",
  "data": [
    {
      "id": 1,
      "prom_id": 1,
      "expr": "node_memory_Active_bytes",
      "op": ">",
      "value": "1000000",
      "for": "120s",
      "labels": {},
      "summary": "内存告警",
      "description": "内存使用率过高"
    }
  ]
}
```

### 2. 获取数据源列表

```
GET /api/v1/proms
Header: Token: <auth_token>

Response:
{
  "code": 0,
  "msg": "success",
  "data": [
    {
      "id": 1,
      "url": "http://prometheus:9090"
    }
  ]
}
```

### 3. 接收告警通知

```
POST /api/v1/alerts
Header: Token: <auth_token>
Content-Type: application/json

Body:
[
  {
    "state": "firing",
    "labels": {...},
    "annotations": {...},
    "value": 1234.56,
    "active_at": "2026-02-03T10:00:00Z",
    "fired_at": "2026-02-03T10:02:00Z"
  }
]
```

## 贡献

欢迎提交 Issue 和 Pull Request！

---
**Made with ❤️ by Will Yin**
