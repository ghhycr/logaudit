# LogAudit - 日志审计平台

> 自建 Web 日志留存查看平台 | 面向网络设备 · 服务器 · 安全设备日志的集中采集、留存、检索与告警审计系统（等保三级）

持续维护中：基于 ELK + ElastAlert2 方案的轻量化自研替代，采用 **Vue 3 自研前端 + Go 自研后端 + ClickHouse 列式存储 + Vector 采集器** 的完整开源技术栈，全部组件开源、前端页面全自研，支持 Ubuntu 24.04 单机/容器化部署。当前已完成 M1~M5 里程碑（前端骨架 → 认证 → 日志检索 → 采集器 → 自研告警引擎），正在持续完善功能与文档，欢迎提交 Issue、建议和 PR。

关键词：`日志审计 | 日志留存 | 安全审计 | 等保三级 | 网络设备日志 | 服务器日志 | 安全设备日志 | syslog | ClickHouse | Vector | 告警引擎 | 开源`

---

## 功能特性

| 模块 | 功能要点 |
|---|---|
| **总览仪表盘** | 今日日志量 / 近 7 天趋势折线图 / 设备接入数 / 留存天数 |
| **日志检索**（核心） | 时间范围、设备、类别、事件类型、严重级别、关键词组合筛选；结果表格 + 原始报文详情抽屉；分页统计 |
| **统计分析** | 登录失败 7 天趋势、事件类别分布饼图、TOP 源 IP、TOP 事件类型 |
| **设备台账** | 设备 CRUD（仅 admin）、启停管理、设备接入引导 |
| **留存与容量** | ClickHouse 各日期分区真实占用、TTL 到期倒计时、存储预估（默认 180 天，等保 ≥6 个月） |
| **告警规则** | 自研告警引擎（等价 ElastAlert2）：frequency / spike / flatline / any 四类规则、分组聚合、realert 去重、Webhook/SMTP 动作、规则试运行 |
| **告警事件** | 告警事件列表、级别/状态筛选、确认处置、统计概览 |
| **系统设置** | 修改密码、账号角色（admin/viewer/auditor 最小权限）、操作审计留痕 |

**等保三级安全框架（前端自研）**：JWT 双令牌（15 分钟 access + 7 天 refresh 可吊销）、bcrypt 口令散列、CSRF 双校验、账号锁定（5 次失败锁 10 分钟）、操作审计队列、XSS 白名单过滤、CSP 安全头、15 分钟空闲登出、角色路由守卫。

---

## 系统架构（五层）

```
┌─────────────────────────────────────────────┐
│  展示层  Vue 3 + TS + Element Plus + ECharts │  ← 全部页面自研
├─────────────────────────────────────────────┤
│  服务层  Go AuditAPI（REST /api/v1/*）       │  ← 自研
│          认证 · 检索 · 统计 · 设备 · 告警引擎 │
├─────────────────────────────────────────────┤
│  存储层  ClickHouse（日志流水, TTL 180d）     │
│          MySQL（用户/会话/操作审计/设备台账） │
├─────────────────────────────────────────────┤
│  采集层  Vector（514/udp+514/tcp syslog）    │
│          客户端 rsyslog / Filebeat 上报       │
├─────────────────────────────────────────────┤
│  接入层  Nginx（TLS 443 · 静态托管 · API 反代）│
└─────────────────────────────────────────────┘
```

- 架构图：`docs/架构图.html`
- M3 数据流：`docs/m3-数据流.html`
- M5 告警引擎验证：`docs/m5-告警引擎.html`

---

## 技术栈（全部开源）

| 组件 | 选型 | 用途 |
|---|---|---|
| 前端 | Vue 3.5 + TypeScript + Vite 6 + Element Plus + ECharts | 展示层（全自研，等保三级安全框架） |
| 后端 | Go 1.22（Gin 路由风格，静态编译 scratch 镜像） | REST API + 告警引擎 |
| 日志存储 | ClickHouse 24.8（MergeTree + ZSTD + ngrambf_v1 索引） | 日志流水，TTL 自动清理 |
| 元数据 | MySQL 8.0 | 用户 / 会话 / 操作审计 / 设备台账 |
| 采集 | Vector 0.44 | syslog 514 UDP/TCP 实时采集 |
| 接入 | Nginx 1.27（TLS 自签/Let's Encrypt） | 静态托管 + API 反向代理 |

---

## 快速开始

### 环境要求

- 操作系统：Ubuntu 24.04 / 26.04 LTS（x86_64）
- 内存 ≥ 4 GB（建议 8 GB），磁盘 ≥ 40 GB（日志按量增长）
- Docker 24+ / docker-compose-v2
- 测试环境：单机即可；生产建议数据盘独立挂载

### 一键安装

```bash
# 1. 获取项目
git clone https://gitee.com/ghhycr/logaudit.git /opt/audit
cd /opt/audit

# 2. 按需修改环境变量（默认已可跑通）
cp deploy/docker/.env.example .env   # 如有

# 3. 后端静态编译（Go 1.22+）
cd backend && CGO_ENABLED=0 go build -o ../deploy/docker/api/audit-api . && cd ..

# 4. 前端构建（Node 20+）
cd web && npm install --registry=https://registry.npmmirror.com && npm run build:no-type && cd ..

# 5. 拉起全部容器
cd deploy/docker && docker compose up -d --build
docker compose restart nginx        # 反代 IP 变更后必须重启 nginx

# 6. 初始化 ClickHouse 表（数据卷非空时手动执行）
docker exec -i audit-clickhouse clickhouse-client --password <CLICKHOUSE_PASSWORD> < clickhouse/init.sql

# 7. 全量验证（10/10 检查项）
bash verify-container.sh
```

### 访问服务

- Web 入口：`https://<服务器IP>/`
- 初始管理员账号：`admin`（首次启动打印随机密码至容器日志，可用 `docker logs audit-api | grep -i password` 查看，随后立即修改）

### 常用命令

```bash
docker compose up -d                 # 启动全部服务
docker compose ps                    # 查看容器状态
docker compose logs -f audit-api     # 后端日志
docker compose logs -f audit-vector  # 采集器日志
docker compose restart nginx         # 重启接入层（反代目标变更后必做）
docker exec -it audit-clickhouse clickhouse-client   # 进入 ClickHouse
```

### 设备接入（新增一台 Linux 主机上报 syslog）

1. 平台「设备台账」→ 新增设备（名称/IP/厂商/类型）
2. 客户端配置 `/etc/rsyslog.d/60-audit.conf`：

```bash
*.* @@192.168.143.131:514   # 平台服务器 IP，TCP 用 @@，UDP 用 @
systemctl restart rsyslog
```

3. 平台「日志检索」中按设备 IP 过滤即可看到实时入库日志。

---

## 告警引擎（等价 ElastAlert2）

- **frequency**：窗口内事件数 ≥ 阈值
- **spike**：当前窗口相对基线窗口突增倍数
- **flatline**：窗口内静默（低于阈值即告警）
- **any**：命中即告警
- 分组聚合（host / source_ip / event_type / user_name）、realert 去重、Webhook / SMTP 动作、规则试运行 dry-run、事件确认处置

---

## 安装手册

详细部署指南（含前置准备、镜像源配置、逐组件部署、设备接入、告警配置、常见问题排查）：[docs/安装手册.md](docs/安装手册.md)

---

## 反馈与贡献

- 提交 [Issue](https://gitee.com/ghhycr/logaudit/issues) 反馈问题与建议
- 欢迎 Pull Request：前端页面（`web/src/views/`）、后端接口（`backend/`）、部署脚本（`deploy/`）
- 安全漏洞请通过 [SECURITY.md](SECURITY.md) 渠道上报

## License

[MIT](LICENSE) © 2026
