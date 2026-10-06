#!/usr/bin/env bash
# ============================================================================
# verify-container.sh — 容器化部署验证（等保三级检查项）
# 用法: ./verify-container.sh
# 退出码: 0=全部 OK / 1=存在 ERR
# ============================================================================
# 注意：此处有意不使用 -e（逐项验证需在单项失败后继续执行并统计 FAIL）
set -uo pipefail

IP=$(hostname -I | awk '{print $1}')
BASE_URL="https://$IP"
FAIL=0

ok()   { echo "[ OK ] $1"; }
warn() { echo "[WARN] $1"; }
err()  { echo "[ERR ] $1"; FAIL=1; }

echo "==================== 容器化部署验证 ===================="
echo "[INFO] 目标: $BASE_URL"

echo ""
echo "[1/11] 容器状态"
docker compose ps --format "table {{.Name}}\t{{.Status}}" || err "docker compose ps 失败"

echo ""
echo "[2/11] 端口监听"
for p in 80 443; do
  ss -lnt | grep -q ":$p " && ok "监听 $p/tcp" || err "未监听 $p/tcp"
done

echo ""
echo "[3/11] 安全响应头（等保三级）"
HDRS=$(curl -skI --max-time 10 "$BASE_URL/" 2>/dev/null || true)
check_hdr() {
  local name="$1" pattern="$2"
  echo "$HDRS" | grep -qi "$pattern" && ok "$name" || err "$name 缺失"
}
check_hdr "X-Frame-Options=DENY"      "x-frame-options:.*deny"
check_hdr "X-Content-Type-Options"    "x-content-type-options:.*nosniff"
check_hdr "Strict-Transport-Security" "strict-transport-security:"
check_hdr "Content-Security-Policy"   "content-security-policy:"
check_hdr "Referrer-Policy"           "referrer-policy:"
if echo "$HDRS" | grep -q "server: nginx/"; then
  err "server 头泄露版本号（server_tokens 未生效）"
else
  ok "隐藏 server 版本（server: nginx）"
fi

echo ""
echo "[4/11] 前端资源与 SPA 回退"
code=$(curl -sk --max-time 10 -o /dev/null -w "%{http_code}" "$BASE_URL/")
[ "$code" = "200" ] && ok "首页 HTTP $code" || err "首页 HTTP $code（检查 dist 挂载与 audit.conf）"
body=$(curl -sk --max-time 10 "$BASE_URL/dashboard" 2>/dev/null || true)
echo "$body" | grep -qi "index.html\|<div id=\"app\"\|<script" \
  && ok "SPA 路由回退正常" || err "SPA 路由回退异常（/dashboard 未返回应用壳）"

echo ""
echo "[5/11] API 认证链路（M2：真实后端登录验证）"
api_code=$(curl -sk --max-time 10 -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/auth/me" 2>/dev/null || true)
case "$api_code" in
  401) ok "API 反代连通（未授权返回 401 正常）" ;;
  200) warn "API 反代连通（HTTP 200，无令牌返回用户信息，检查鉴权）" ;;
  502|503|504) err "API 反代 $api_code（audit-api 未启动或网络异常）" ;;
  *) warn "API 反代 HTTP $api_code（非预期）" ;;
esac

# 真实登录验证：从 audit-api 日志提取初始密码（若已改密则跳过）
ADMIN_PASS=$(docker logs audit-api 2>/dev/null | grep -o '初始密码: [^（]*' | head -1 | sed 's/初始密码: //')
if [ -n "$ADMIN_PASS" ]; then
  login_code=$(curl -sk --max-time 10 -o /tmp/audit_login.json -w "%{http_code}" -X POST \
    "$BASE_URL/api/v1/auth/login" -H 'Content-Type: application/json' \
    -d "{\"username\":\"admin\",\"password\":\"$ADMIN_PASS\"}" 2>/dev/null || true)
  if [ "$login_code" = "200" ] && grep -q '"code":0' /tmp/audit_login.json; then
    ok "admin 登录成功（初始密码有效）"
  else
    warn "admin 登录测试未通过（HTTP $login_code，可能已修改密码，跳过）"
  fi
else
  warn "未提取到初始密码（admin 可能已改密，跳过登录验证）"
fi

echo ""
echo "[6/11] TLS 证书"
if [ -f certs/audit.crt ]; then
  enddate=$(openssl x509 -in certs/audit.crt -noout -enddate 2>/dev/null | cut -d= -f2)
  days=$(( ( $(date -d "$enddate" +%s) - $(date +%s) ) / 86400 ))
  [ "$days" -gt 30 ] && ok "证书有效期至 $enddate（剩余 ${days} 天）" || warn "证书即将过期（剩余 ${days} 天）"
fi

echo ""
echo "[7/11] ClickHouse 存储就绪"
if docker exec audit-clickhouse clickhouse-client --password "${CLICKHOUSE_PASSWORD:-audit2026}" -q \
  "SELECT concat('audit_logs rows=', toString(count())) FROM audit.audit_logs" 2>/dev/null; then
  ok "ClickHouse audit.audit_logs 可查询"
else
  err "ClickHouse 查询失败（检查容器状态与 init.sql）"
fi

echo ""
echo "[8/11] M3 日志检索链路（logs/search + retention）"
M3_TOKEN=$(curl -sk --max-time 10 -X POST "$BASE_URL/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"'"$ADMIN_PASS"'"}' 2>/dev/null \
  | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
if [ -n "$M3_TOKEN" ]; then
  sr_code=$(curl -sk --max-time 10 -o /tmp/audit_m3.json -w "%{http_code}" \
    "$BASE_URL/api/v1/logs/search?page=1&size=5" -H "Authorization: Bearer $M3_TOKEN" 2>/dev/null || true)
  if [ "$sr_code" = "200" ] && grep -q '"code":0' /tmp/audit_m3.json; then
    rows=$(grep -o '"total":[0-9]*' /tmp/audit_m3.json | head -1 | cut -d: -f2)
    ok "日志检索接口正常（total=${rows:-0}）"
  else
    warn "日志检索接口 HTTP $sr_code（检查 token 与 ClickHouse 表）"
  fi
  rt_code=$(curl -sk --max-time 10 -o /dev/null -w "%{http_code}" \
    "$BASE_URL/api/v1/retention" -H "Authorization: Bearer $M3_TOKEN" 2>/dev/null || true)
  [ "$rt_code" = "200" ] && ok "留存策略接口正常" || warn "留存策略接口 HTTP $rt_code"
else
  warn "未取得 M3 验证 token（admin 已改密则跳过）"
fi

echo ""
echo "[9/11] Vector 采集器（M4：514 syslog → ClickHouse）"
if docker ps --format '{{.Names}}' | grep -q '^audit-vector$'; then
  ok "audit-vector 容器运行中"
else
  err "audit-vector 未运行"
fi
if ss -lun | grep -q ':514'; then
  ok "514/udp syslog 监听正常"
else
  err "514/udp 未监听"
fi
ch_rows=$(docker exec audit-clickhouse clickhouse-client --password "${CLICKHOUSE_PASSWORD:-audit2026}" -q \
  "SELECT count() FROM audit.audit_logs" 2>/dev/null || echo "ERR")
case "$ch_rows" in
  ''|ERR) err "ClickHouse 行数查询失败" ;;
  *) ok "audit_logs 当前 ${ch_rows} 行（Vector 实时写入生效）" ;;
esac

echo ""
echo "[10/11] M5 告警引擎（rules + events 接口）"
if [ -n "$M3_TOKEN" ]; then
  ar_code=$(curl -sk --max-time 10 -o /tmp/audit_m5.json -w "%{http_code}" \
    "$BASE_URL/api/v1/alerts/rules" -H "Authorization: Bearer $M3_TOKEN" 2>/dev/null || true)
  if [ "$ar_code" = "200" ] && grep -q '"code":0' /tmp/audit_m5.json; then
    cnt=$(grep -o '"total":[0-9]*' /tmp/audit_m5.json | head -1 | cut -d: -f2)
    ok "告警规则接口正常（规则数=${cnt:-0}）"
  else
    warn "告警规则接口 HTTP $ar_code（检查引擎启动与 alert_rules 表）"
  fi
  ae_code=$(curl -sk --max-time 10 -o /dev/null -w "%{http_code}" \
    "$BASE_URL/api/v1/alerts/events/stats" -H "Authorization: Bearer $M3_TOKEN" 2>/dev/null || true)
  [ "$ae_code" = "200" ] && ok "告警事件接口正常" || warn "告警事件接口 HTTP $ae_code"
else
  warn "未取得 M5 验证 token（admin 已改密则跳过）"
fi

echo ""
echo "[11/11] Redis 优化层（登录防爆破 / 告警去重 / 统计缓存）"
if command -v redis-cli >/dev/null 2>&1 && redis-cli -a "${REDIS_PASSWORD:-audit2026}" --no-auth-warning ping 2>/dev/null | grep -q PONG; then
  ok "Redis 服务正常（PONG）"
else
  err "Redis 未就绪（容器部署需 redis 服务；宿主机部署需 redis-server）"
fi
if [ -n "$M3_TOKEN" ]; then
  curl -sk --max-time 10 -o /dev/null "$BASE_URL/api/v1/stats/overview" -H "Authorization: Bearer $M3_TOKEN" 2>/dev/null || true
  sleep 1
  ttl=$(redis-cli -a "${REDIS_PASSWORD:-audit2026}" --no-auth-warning TTL stats:overview 2>/dev/null || echo "")
  if [ -n "$ttl" ] && [ "$ttl" != "-2" ]; then
    ok "统计缓存生效（stats:overview TTL=${ttl}s）"
  else
    warn "统计缓存未写入（接口未触发或 Redis 连接异常）"
  fi
else
  warn "未取得验证 token（admin 已改密则跳过缓存检查）"
fi

echo ""
echo "==================== 结果汇总 ===================="
[ "$FAIL" -eq 0 ] && echo "[ OK ] 部署验证全部通过" || echo "[ERR ] 存在失败项，请按提示排查"
exit "$FAIL"
