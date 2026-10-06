# 日志审计平台 · 部署（Nginx + systemd，Ubuntu 24.04）

前端（Vue3 静态产物）由 **Nginx** 托管并反向代理后端 API，后端 API（Spring Boot / Go）由 **systemd** 托管。全部为开源组件 + 自研代码。

## 目录结构

```
deploy/
├── README.md                    # 本文档
├── nginx/
│   └── audit.conf.tpl           # Nginx 站点配置模板（__X__ 占位符由脚本替换）
├── systemd/
│   └── audit-api.service        # 后端 API systemd 单元（Spring Boot / Go 通用）
└── scripts/
    ├── install-nginx.sh         # 安装 Nginx + 生成站点配置 + 启用 + 证书
    ├── tls-setup.sh             # TLS 证书管理（自签 10 年 / Let's Encrypt / 续期 / 检查）
    ├── deploy-frontend.sh       # 前端产物部署（本机 cp / 远程 rsync，自动备份旧版）
    └── verify.sh                # 部署验证（等保三级检查项）
```

## 部署流程（目标机 Ubuntu 24.04，全程 root）

```bash
# 1) 部署文件上传到服务器（如 /opt/audit/deploy）后进入目录
cd /opt/audit/deploy/scripts

# 2) 安装 Nginx 并生成站点（内网自签模式）
./install-nginx.sh --domain audit.corp.cn --tls-mode self

# 3) 部署前端构建产物（在开发机 npm run build 后，产物 web/dist/ 上传到服务器）
./deploy-frontend.sh --src /path/to/web/dist

# 4) 部署后端 API（Spring Boot jar 或 Go 二进制）
mkdir -p /opt/audit/backend
useradd -r -s /usr/sbin/nologin audit
cp /path/to/audit-api.jar /opt/audit/backend/
cp ../systemd/audit-api.service /etc/systemd/system/
systemctl daemon-reload && systemctl enable --now audit-api

# 5) 验证
./verify.sh
```

### 域名（公网）+ Let's Encrypt 模式

```bash
./install-nginx.sh --domain audit.example.com \
  --tls-mode letsencrypt --certbot-email admin@example.com
# 证书自动续期由 certbot 定时任务完成；手动续期: ./tls-setup.sh --renew
```

### 远程一键部署前端（开发机执行，服务器已跑 install-nginx.sh）

```bash
./deploy-frontend.sh --src ./web/dist --host root@server --dest /opt/audit/web/dist
# 要求开发机到服务器已配置 SSH 免密，且开发机装有 rsync
```

## 等保三级检查项对照（部署层）

| 等保 2.0 三级要求 | 落地位置 |
|---|---|
| 身份鉴别 / 数据保密 | HTTPS（TLS 1.2+，HTTP 强制 301 跳转）、HSTS |
| 入侵防范 | CSP、X-Frame-Options=DENY、nosniff、Referrer-Policy、隐藏 server 版本 |
| 安全审计 | nginx access_log 独立文件（audit_access.log）、后端操作审计表 |
| 访问控制 / 最小权限 | Nginx 仅暴露 80/443；后端以非特权用户 audit 运行（NoNewPrivileges/PrivateTmp/ProtectSystem） |
| 资源控制 | 会话超时（前端 15 分钟）、后端限流（部署时按需加 limit_req） |

> CSP 说明：`style-src 'unsafe-inline'` 为 Element Plus 动态样式所必需；如需更严格可改为 nonce 方案（自研后端 M2 阶段统一处理）。

## 常见问题

- **脚本报 `\r` 错误**：Windows 上编辑过的脚本带 CRLF，部署前执行
  `sed -i 's/\r$//' install-nginx.sh tls-setup.sh deploy-frontend.sh verify.sh`
- **verify.sh 提示 API 502**：Nginx 正常，后端服务未启动（`systemctl status audit-api`、`ss -lntp | grep 8080`）
- **自签证书浏览器告警**：内网正常现象；生产域名请用 Let's Encrypt 或内网 CA
- **前端 404**：`web/dist` 未部署或部署到了错误的 `--dest`，运行 `deploy-frontend.sh` 后 `verify.sh` 复验
