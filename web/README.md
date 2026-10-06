# AuditWeb · 前端骨架工程

自建 Web 日志留存查看平台前端（等保三级安全框架），Vue 3 + TypeScript + Vite + Pinia + Element Plus + ECharts。

## 快速运行

```bash
cd web
npm install
npm run dev          # http://localhost:5173（开发代理 /api → 127.0.0.1:8080）
```

构建：

```bash
npm run build        # 类型检查 + 产物 dist/
npm run preview      # 本地预览构建产物
```

## 目录结构

```
web/
├── index.html                    # CSP / X-Frame-Options / nosniff 安全头
├── vite.config.ts                # 开发代理 / 分包构建
├── .env.development / .env.production   # 会话与密码策略配置
└── src/
    ├── main.ts                   # 入口：注册 router/pinia/指令
    ├── App.vue                   # 根组件（启动会话监控）
    ├── router/index.ts           # 路由守卫（登录校验 + 角色权限）
    ├── stores/auth.ts            # 认证状态（token 内存存储、锁定计数）
    ├── api/                      # axios 封装（JWT/401 刷新/CSRF）+ 各域 API
    ├── utils/                    # password/session/security/audit 安全工具
    ├── directives/               # v-perm / v-audit / v-sanitize 指令
    ├── components/
    │   ├── layout/AppLayout.vue  # 侧边栏 + 头部 + 会话提示 + 登出
    │   └── security/PasswordStrength.vue  # 密码强度条
    ├── views/                    # 登录 / 仪表盘 / 检索 / 统计 / 设备 / 留存 / 设置 / 403 / 404
    └── types/                    # 全局类型
```

## 等保三级安全实现对照

| 等保 2.0 三级要求 | 前端实现位置 |
|---|---|
| **身份鉴别**：密码复杂度（≥8 位大小写数字特殊）、登录失败锁定、会话超时 | `utils/password.ts`（强度校验）、`stores/auth.ts`（失败计数+锁定窗口）、`utils/session.ts`（空闲 15 分钟自动登出 + 提前提示） |
| **访问控制**：最小权限、角色分离 | `router/index.ts`（路由 meta.roles 守卫）、`directives/index.ts` v-perm（按钮级）、角色 admin/viewer/auditor |
| **安全审计**：操作留痕 | `utils/audit.ts`（操作审计队列，登录/查询/导出/管理动作埋点）、`directives` v-audit、登出/超时前强制 flush |
| **入侵防范**：XSS/CSRF/注入防护 | `index.html` CSP + X-Frame-Options + nosniff；`utils/security.ts` 白名单 XSS 清洗；v-sanitize 指令；写请求带 X-XSRF-TOKEN；关键词长度限制 |
| **数据保密性**：HTTPS、敏感数据最小化 | token 存内存不落 localStorage；`.env.production` 同域反代；生产部署要求 Caddy/Nginx HTTPS |
| **会话管理**：超时失效、登出清理 | `utils/session.ts` 空闲超时；`stores/auth.ts` logout 清空全部会话态；401 自动刷新 |

## 与后端（AuditAPI）对接契约（骨架阶段）

前端已按契约封装 API，后端需提供：

- `POST /api/v1/auth/login` → `{ access_token, refresh_token, expires_in, csrf_token?, user }`
- `POST /api/v1/auth/refresh` → `{ access_token }`
- `POST /api/v1/auth/logout`、`POST /api/v1/auth/change-password`、`GET /api/v1/auth/me`
- `GET /api/v1/logs/search`、`GET /api/v1/logs/{id}`、`POST /api/v1/logs/export`
- `GET /api/v1/stats/overview`、`GET /api/v1/stats/login-fail`
- `GET/POST/PUT/DELETE /api/v1/devices`、`GET /api/v1/retention`、`GET /api/v1/users`
- `POST /api/v1/audit/ops`（操作审计上报）

登录失败锁定：**以服务端为准**（返回 `locked` 状态/错误码），前端展示锁定提示与倒计时。
