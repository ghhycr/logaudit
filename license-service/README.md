# 授权管理模块 License Service（Java）

日志审计平台「系统设置 → 授权管理」的后端服务，使用 **Java 17 + Spring Boot 3** 实现，容器化部署在 `audit-license` 容器中（Nginx 反代 `/api/v1/license/` → `license:8091`）。

## 能力

| 能力 | 说明 |
|---|---|
| 服务器硬件指纹 | 采集 `/etc/machine-id`、DMI `product_uuid`、主板序列号、网卡 MAC，SHA-256 后格式化为 `LGA-XXXXXXXX-XXXXXXXX-XXXXXXXX-XXXXXXXX`，作为授权绑定依据 |
| 离线签发（LicenseTool.exe） | 授权码**一律由本地工具离线签发**，本服务不提供在线签发接口；页面展示 4 步操作指引与签名密钥，管理员复制硬件指纹后在工具中签发 |
| 授权码/授权文件校验 | HMAC-SHA256 签名防篡改、有效期校验、硬件绑定比对（防跨机复制） |
| 授权导入激活 | 粘贴授权码或上传 `.lic` 授权文件 → 校验 → 写入 `license_active` 生效，同时写入导入留痕 |
| 复制授权码 | 前端一键复制（平台内与 `LicenseTool.exe` 通用） |
| 导出授权文件 | 导出 `.lic` 授权文件（首行授权码 + 产品/硬件/类型/到期元信息），支持按导入记录重新导出 |
| 授权导入记录 | MySQL `license_issue_log` 记录操作人、时间、客户、绑定硬件（等保三级留痕） |
| 访问控制 | 复用平台 JWT（HS256，同一把 `JWT_SECRET`），写操作要求 `admin` 角色 |

> 变更说明：2026-10-07 起取消平台在线签发（`POST /license/issue` 已下线），
> 授权码统一由 `LicenseTool.exe` 使用同一签名密钥离线签发后在平台导入激活，
> 避免签发密钥暴露在服务端接口中。

## 授权码协议（与 LicenseTool.exe 完全一致）

```
LGA2.<base64url(payload)>.<hex(hmac_sha256(secret, base64url(payload)))>

payload = {"hw": "<硬件指纹>", "perm": true|false, "iat": <unix>, "exp": "YYYY-MM-DD"}
```

- 签名密钥 `LICENSE_SECRET` 必须与本地授权工具 `LicenseTool.exe` 顶部「签名密钥」一致，
  由 `deploy/docker/.env` 注入（compose 中声明为必填，未提供将拒绝启动）。
  **密钥不写入代码仓库**，README 与代码中均不保存明文。
- 授权文件内容首行须为授权码，其后 `#KEY: value` 形式的元信息注释；`LicenseTool.exe` 读取首行校验。

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/license/health` | 健康检查（无需鉴权） |
| GET | `/api/v1/license/hardware` | 硬件指纹与硬件明细 |
| GET | `/api/v1/license/status` | 当前授权状态 |
| POST | `/api/v1/license/verify` | 校验授权码 / 授权文件内容（不落库） |
| POST | `/api/v1/license/activate` | 导入并激活（校验硬件绑定） |
| POST | `/api/v1/license/deactivate` | 解除当前授权 |
| GET | `/api/v1/license/records` | 授权导入记录（admin） |
| GET | `/api/v1/license/export/{id}` | 导出指定导入记录的 `.lic` |
| GET | `/api/v1/license/export-active` | 导出当前生效授权的 `.lic` |

统一响应：`{"code":0,"message":"ok","data":{...}}`，失败 `{"code":1,"message":"原因"}`。

## 数据表（MySQL `audit_meta`，服务启动自动建表）

- `license_active`：当前生效授权（单条 `id=1`）
- `license_issue_log`：授权导入留痕（导入激活时写入，可按记录重新导出 `.lic`）

## 构建与部署

```bash
# 1) 编译（宿主机 Maven；与 Go 后端「宿主机编译 + 镜像打包」约定一致）
cd license-service && mvn -B -ntp package -DskipTests

# 2) 构建镜像
docker build -t logaudit/license-svc .

# 3) 随平台启动
cd ../deploy/docker && docker compose up -d --build license
```

## 配置项（`deploy/docker/.env`）

| 变量 | 说明 |
|---|---|
| `LICENSE_SECRET` | 授权码签名密钥（与 LicenseTool.exe 一致，必填，从 `.env` 注入） |
| `JWT_SECRET` | 与 Go 后端相同的 JWT 密钥，用于校验访问令牌 |
| `MYSQL_PASSWORD` | MySQL `audit` 用户密码（授权数据存储） |

## 目录结构

```
license-service/
├── Dockerfile                                   # eclipse-temurin:17-jre 运行环境
├── pom.xml
└── src/main/
    ├── java/com/logaudit/license/
    │   ├── LicenseServiceApplication.java
    │   ├── core/LicenseCodec.java               # 授权码编解码（HMAC-SHA256）
    │   ├── core/HardwareFingerprint.java        # 服务器硬件指纹采集
    │   ├── core/JwtVerifier.java                # JWT(HS256) 校验（纯 JDK 实现）
    │   ├── data/LicenseRepository.java          # MySQL 数据访问
    │   └── web/LicenseController.java           # REST 接口
    └── resources/application.yml
```
