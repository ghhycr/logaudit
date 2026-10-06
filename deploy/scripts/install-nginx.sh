#!/usr/bin/env bash
# ============================================================================
# install-nginx.sh — 日志审计平台前端部署：安装 Nginx、生成站点配置、启用站点
# 目标系统: Ubuntu 24.04（需 root）
#
# 用法:
#   ./install-nginx.sh [选项]
#     --domain example.com         站点域名或 IP（默认: 主机名）
#     --dist /opt/audit/web/dist   前端构建产物目录（默认如上）
#     --api  http://127.0.0.1:8080 后端 API 上游地址（默认如上）
#     --tls-mode self|letsencrypt  证书模式（默认 self）
#     --certbot-email a@b.c        letsencrypt 模式必填
#     -h|--help                    显示帮助
#
# 示例:
#   ./install-nginx.sh --domain audit.corp.cn --tls-mode self
#   ./install-nginx.sh --domain audit.corp.cn --tls-mode letsencrypt --certbot-email admin@corp.cn
# ============================================================================
set -euo pipefail

DOMAIN="${DOMAIN:-$(hostname -f)}"
DIST_DIR="${DIST_DIR:-/opt/audit/web/dist}"
API_UPSTREAM="${API_UPSTREAM:-http://127.0.0.1:8080}"
TLS_CERT="${TLS_CERT:-/etc/nginx/ssl/audit.crt}"
TLS_KEY="${TLS_KEY:-/etc/nginx/ssl/audit.key}"
TLS_MODE="${TLS_MODE:-self}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TPL="${SCRIPT_DIR}/../nginx/audit.conf.tpl"
SITE_CONF="/etc/nginx/sites-available/audit"

usage() {
  cat <<EOF
用法: $0 [选项]
  --domain example.com         站点域名或 IP（默认: 主机名）
  --dist /opt/audit/web/dist   前端构建产物目录
  --api  http://127.0.0.1:8080 后端 API 上游地址
  --tls-mode self|letsencrypt  证书模式（默认 self）
  --certbot-email a@b.c        letsencrypt 模式必填
  -h|--help                    显示帮助
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain)        DOMAIN="$2"; shift 2 ;;
    --dist)          DIST_DIR="$2"; shift 2 ;;
    --api)           API_UPSTREAM="$2"; shift 2 ;;
    --tls-mode)      TLS_MODE="$2"; shift 2 ;;
    --certbot-email) CERTBOT_EMAIL="$2"; shift 2 ;;
    -h|--help)       usage; exit 0 ;;
    *) echo "[ERR] 未知参数: $1"; usage; exit 1 ;;
  esac
done

# ---------- 前置检查 ----------
[ "$(id -u)" -eq 0 ] || { echo "[ERR] 请以 root 运行本脚本"; exit 1; }
[ -f "$TPL" ] || { echo "[ERR] 模板不存在: $TPL"; exit 1; }
case "$TLS_MODE" in
  self|letsencrypt) ;;
  *) echo "[ERR] --tls-mode 仅支持 self|letsencrypt"; exit 1 ;;
esac

echo "[INFO] 站点域名: $DOMAIN  证书模式: $TLS_MODE"
echo "[INFO] 前端目录: $DIST_DIR   API 上游: $API_UPSTREAM"

# ---------- 1. 安装 Nginx ----------
echo "[1/6] 安装 nginx ..."
apt-get update -y
apt-get install -y nginx openssl

# ---------- 2. 准备 TLS 证书 ----------
echo "[2/6] 准备 TLS 证书（模式: $TLS_MODE）"
mkdir -p /etc/nginx/ssl

gen_selfsigned() {
  # 域名或 IP 分别生成对应 SAN
  if [[ "$DOMAIN" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]]; then
    SAN="IP:$DOMAIN"
  else
    SAN="DNS:$DOMAIN"
  fi
  openssl req -x509 -nodes -newkey rsa:2048 -days 3650 \
    -keyout "$TLS_KEY" -out "$TLS_CERT" \
    -subj "/CN=$DOMAIN" -addext "subjectAltName=$SAN" \
    -addext "extendedKeyUsage=serverAuth" 2>/dev/null
  echo "[ OK ] 已生成自签证书: $TLS_CERT（有效期 10 年，内网使用）"
}

if [ "$TLS_MODE" = "self" ]; then
  if [ ! -f "$TLS_CERT" ] || [ ! -f "$TLS_KEY" ]; then
    gen_selfsigned
  else
    echo "[INFO] 证书已存在，跳过生成: $TLS_CERT"
  fi
else
  # letsencrypt 模式：先用临时自签保证站点可启动，certbot 签发后自动替换
  [ -n "$CERTBOT_EMAIL" ] || { echo "[ERR] letsencrypt 模式必须提供 --certbot-email"; exit 1; }
  gen_selfsigned
fi

# ---------- 3. 生成站点配置 ----------
echo "[3/6] 生成站点配置 ..."
mkdir -p "$(dirname "$SITE_CONF")"
sed -e "s|__DOMAIN__|$DOMAIN|g" \
    -e "s|__TLS_CERT__|$TLS_CERT|g" \
    -e "s|__TLS_KEY__|$TLS_KEY|g" \
    -e "s|__DIST_ROOT__|$DIST_DIR|g" \
    -e "s|__API_UPSTREAM__|$API_UPSTREAM|g" \
    "$TPL" > "$SITE_CONF"

# ---------- 4. 启用站点 ----------
echo "[4/6] 启用站点并移除默认站点 ..."
ln -sf "$SITE_CONF" /etc/nginx/sites-enabled/audit
rm -f /etc/nginx/sites-enabled/default

# 前端目录预创建（deploy-frontend.sh 实际拷贝产物）
mkdir -p "$DIST_DIR"
chown -R www-data:www-data "$(dirname "$DIST_DIR")" 2>/dev/null || true

# ---------- 5. 语法检查并重载 ----------
echo "[5/6] nginx -t 语法检查 ..."
if ! nginx -t; then
  echo "[ERR] nginx 配置语法错误，请检查 $SITE_CONF"
  exit 1
fi
systemctl enable --now nginx
systemctl reload nginx
echo "[ OK ] nginx 已重载"

# ---------- 6. letsencrypt 签发（可选） ----------
if [ "$TLS_MODE" = "letsencrypt" ]; then
  echo "[6/6] certbot 签发 Let's Encrypt 证书 ..."
  apt-get install -y certbot python3-certbot-nginx
  certbot --nginx -d "$DOMAIN" -m "$CERTBOT_EMAIL" \
    --agree-tos --no-eff-email --redirect --keep-until-expiring
  echo "[ OK ] Let's Encrypt 证书已签发（certbot 已自动更新 $SITE_CONF 的证书路径）"
else
  echo "[6/6] 跳过 certbot（self 模式）"
fi

echo ""
echo "==================== 完成 ===================="
echo "下一步: 运行 deploy-frontend.sh 部署前端产物，再运行 verify.sh 验证"
echo "  ./deploy-frontend.sh --src /path/to/web/dist --host user@server"
echo "  ./verify.sh"
