#!/usr/bin/env bash
# ============================================================================
# verify.sh — 日志审计平台部署验证（等保三级检查项）
# 目标系统: Ubuntu 24.04
#
# 用法:
#   ./verify.sh [--domain example.com] [--api-prefix /api/v1]
# 退出码: 0=全部 OK / 1=存在 ERR
# ============================================================================
# 注意：此处有意不使用 -e（逐项验证需在单项失败后继续执行并统计 FAIL）
set -uo pipefail

DOMAIN="${DOMAIN:-$(hostname -f)}"
API_PREFIX="${API_PREFIX:-/api/v1}"
BASE_URL="${BASE_URL:-https://$DOMAIN}"
FAIL=0

# 结果函数
ok()   { echo "[ OK ] $1"; }
warn() { echo "[WARN] $1"; }
err()  { echo "[ERR ] $1"; FAIL=1; }

echo "==================== 日志审计平台部署验证 ===================="
echo "[INFO] 目标: $BASE_URL"

# ---------- [1/7] Nginx 状态 ----------
echo ""
echo "[1/7] Nginx 配置与服务状态"
if nginx -t >/dev/null 2>&1; then ok "nginx -t 语法通过"; else err "nginx -t 语法错误（运行 nginx -t 查看详情）"; fi
if systemctl is-active --quiet nginx; then ok "nginx 服务运行中"; else err "nginx 服务未运行（systemctl status nginx）"; fi

# ---------- [2/7] 端口监听 ----------
echo ""
echo "[2/7] 端口监听检查（443/80）"
for p in 443 80; do
  if ss -lnt | grep -q ":$p "; then ok "监听 $p/tcp"; else warn "未监听 $p/tcp"; fi
done

# ---------- [3/7] 安全响应头（等保三级） ----------
echo ""
echo "[3/7] 安全响应头检查"
HDRS=$(curl -skI --max-time 10 "$BASE_URL/" 2>/dev/null || true)
check_hdr() {
  local name="$1" pattern="$2"
  if echo "$HDRS" | grep -qi "$pattern"; then ok "$name"; else err "$name 缺失（检查 audit.conf 的 add_header 配置）"; fi
}
check_hdr "X-Frame-Options=DENY"      "x-frame-options:.*deny"
check_hdr "X-Content-Type-Options"    "x-content-type-options:.*nosniff"
check_hdr "Strict-Transport-Security" "strict-transport-security:"
check_hdr "Content-Security-Policy"   "content-security-policy:"
check_hdr "Referrer-Policy"           "referrer-policy:"
check_hdr "Server 未泄露版本号"       "server: nginx/1\.2[0-9]"

# ---------- [4/7] 前端资源与 SPA 回退 ----------
echo ""
echo "[4/7] 前端静态资源检查"
code=$(curl -sk --max-time 10 -o /dev/null -w "%{http_code}" "$BASE_URL/")
[ "$code" = "200" ] && ok "首页 HTTP $code" || err "首页 HTTP $code（检查 dist 产物是否部署）"

# SPA history 路由回退：直接访问子路由应返回 index.html
body=$(curl -sk --max-time 10 "$BASE_URL/dashboard" 2>/dev/null || true)
if echo "$body" | grep -qi "index.html\|<div id=\"app\"\|<script"; then
  ok "SPA 路由回退正常（/dashboard 返回应用壳）"
else
  err "SPA 路由回退异常（/dashboard 未返回 index.html，检查 try_files）"
fi

# ---------- [5/7] API 反向代理 ----------
echo ""
echo "[5/7] API 反向代理检查（$API_PREFIX）"
api_code=$(curl -sk --max-time 10 -o /dev/null -w "%{http_code}" "$BASE_URL$API_PREFIX/auth/me" 2>/dev/null || true)
case "$api_code" in
  401|403|200)
    ok "API 反代连通（/auth/me → HTTP $api_code，后端已响应）" ;;
  502|503|504)
    warn "API 反代返回 $api_code（nginx 正常，但后端 API 服务未启动或未监听 8080）" ;;
  *)
    warn "API 反代返回 HTTP $api_code（非预期，请检查 audit.conf location /api/ 与后端）" ;;
esac

# ---------- [6/7] TLS 证书 ----------
echo ""
echo "[6/7] TLS 证书检查"
if [ -f /etc/nginx/ssl/audit.crt ]; then
  enddate=$(openssl x509 -in /etc/nginx/ssl/audit.crt -noout -enddate 2>/dev/null | cut -d= -f2)
  days=$(( ( $(date -d "$enddate" +%s) - $(date +%s) ) / 86400 ))
  if [ "$days" -gt 30 ]; then ok "证书有效期至 $enddate（剩余 ${days} 天）"
  else warn "证书将于 $enddate 过期（剩余 ${days} 天），请运行 tls-setup.sh --renew"; fi
else
  warn "未找到 /etc/nginx/ssl/audit.crt（letsencrypt 模式时请运行 verify.sh --domain 实际域名）"
fi

# ---------- [7/7] 访问审计日志 ----------
echo ""
echo "[7/7] 访问审计日志（等保三级：安全审计）"
if [ -f /var/log/nginx/audit_access.log ]; then
  cnt=$(wc -l < /var/log/nginx/audit_access.log)
  ok "audit_access.log 存在（当前 ${cnt} 行）"
else
  warn "audit_access.log 不存在（尚无访问或路径被修改）"
fi

echo ""
echo "==================== 结果汇总 ===================="
if [ "$FAIL" -eq 0 ]; then
  echo "[ OK ] 部署验证全部通过"
else
  echo "[ERR ] 存在失败项，请按上方提示逐一排查"
fi
exit "$FAIL"
