#!/usr/bin/env bash
# ============================================================================
# tls-setup.sh — 日志审计平台 TLS 证书管理（自签 / Let's Encrypt）
# 目标系统: Ubuntu 24.04（需 root）
#
# 用法:
#   ./tls-setup.sh --mode self [--domain example.com] [--cert /etc/nginx/ssl/audit.crt] [--key /etc/nginx/ssl/audit.key]
#   ./tls-setup.sh --mode letsencrypt --domain example.com --certbot-email admin@corp.cn
#   ./tls-setup.sh --renew              # certbot 续期（letsencrypt 模式）
#   ./tls-setup.sh --check [--domain example.com]   # 检查证书有效期
#
# 说明:
#   - self 模式生成 10 年自签证书（含 SAN），供内网/测试使用
#   - letsencrypt 模式通过 certbot --nginx 签发并自动改写站点配置
# ============================================================================
set -euo pipefail

DOMAIN="${DOMAIN:-$(hostname -f)}"
TLS_CERT="${TLS_CERT:-/etc/nginx/ssl/audit.crt}"
TLS_KEY="${TLS_KEY:-/etc/nginx/ssl/audit.key}"
MODE="${MODE:-}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-}"
ACTION="${ACTION:-issue}"

usage() {
  cat <<EOF
用法: $0 [选项]
  --mode self|letsencrypt    证书模式（必填，renew/check 时可不填）
  --domain example.com       域名或 IP（默认: 主机名）
  --cert PATH                证书路径（默认 /etc/nginx/ssl/audit.crt）
  --key PATH                 私钥路径（默认 /etc/nginx/ssl/audit.key）
  --certbot-email a@b.c      letsencrypt 模式必填
  --renew                    续期 Let's Encrypt 证书
  --check                    仅检查证书有效期
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode)           MODE="$2"; shift 2 ;;
    --domain)         DOMAIN="$2"; shift 2 ;;
    --cert)           TLS_CERT="$2"; shift 2 ;;
    --key)            TLS_KEY="$2"; shift 2 ;;
    --certbot-email)  CERTBOT_EMAIL="$2"; shift 2 ;;
    --renew)          ACTION="renew"; shift ;;
    --check)          ACTION="check"; shift ;;
    -h|--help)        usage; exit 0 ;;
    *) echo "[ERR] 未知参数: $1"; usage; exit 1 ;;
  esac
done

[ "$(id -u)" -eq 0 ] || { echo "[ERR] 请以 root 运行本脚本"; exit 1; }

# ---------- 检查证书有效期 ----------
check_cert() {
  if [ ! -f "$TLS_CERT" ]; then
    echo "[WARN] 证书不存在: $TLS_CERT"
    return 1
  fi
  openssl x509 -in "$TLS_CERT" -noout -subject -enddate
  days_left=$(openssl x509 -in "$TLS_CERT" -noout -enddate | sed 's/notAfter=//' | xargs -I{} date -d {} +%s | xargs -I{} sh -c 'echo $(( ({} - $(date +%s)) / 86400 ))')
  echo "[INFO] 证书剩余: ${days_left} 天"
  if [ "${days_left:-0}" -lt 30 ]; then
    echo "[WARN] 证书即将过期，请续期（letsencrypt 模式运行 --renew）"
  fi
}

case "$ACTION" in
  check)
    check_cert
    exit 0
    ;;
  renew)
    [ -n "$DOMAIN" ] || { echo "[ERR] --renew 需要 --domain"; exit 1; }
    command -v certbot >/dev/null || { echo "[ERR] 未安装 certbot，请先运行 --mode letsencrypt 或 apt install certbot"; exit 1; }
    certbot renew --nginx
    echo "[ OK ] 续期完成（certbot 定时任务将自动续期）"
    exit 0
    ;;
  issue) ;;
  *) usage; exit 1 ;;
esac

[ -n "$MODE" ] || { echo "[ERR] 缺少 --mode"; usage; exit 1; }
case "$MODE" in
  self|letsencrypt) ;;
  *) echo "[ERR] --mode 仅支持 self|letsencrypt"; exit 1 ;;
esac

mkdir -p "$(dirname "$TLS_CERT")"

if [ "$MODE" = "self" ]; then
  if [[ "$DOMAIN" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]]; then
    SAN="IP:$DOMAIN"
  else
    SAN="DNS:$DOMAIN"
  fi
  openssl req -x509 -nodes -newkey rsa:2048 -days 3650 \
    -keyout "$TLS_KEY" -out "$TLS_CERT" \
    -subj "/CN=$DOMAIN" -addext "subjectAltName=$SAN" \
    -addext "extendedKeyUsage=serverAuth" 2>/dev/null
  echo "[ OK ] 自签证书已生成: $TLS_CERT（有效期 10 年）"
  check_cert
else
  [ -n "$CERTBOT_EMAIL" ] || { echo "[ERR] letsencrypt 模式必须提供 --certbot-email"; exit 1; }
  command -v certbot >/dev/null || { echo "[INFO] 安装 certbot ..."; apt-get update -y; apt-get install -y certbot python3-certbot-nginx; }
  # 依赖 nginx 站点已启用（install-nginx.sh 第 3-4 步）；certbot --nginx 自动改写证书路径
  certbot --nginx -d "$DOMAIN" -m "$CERTBOT_EMAIL" \
    --agree-tos --no-eff-email --redirect --keep-until-expiring
  echo "[ OK ] Let's Encrypt 证书已签发"
fi

echo ""
echo "提示: 运行 deploy/scripts/verify.sh 验证部署；letsencrypt 证书由 certbot 定时任务自动续期。"
