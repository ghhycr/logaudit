#!/usr/bin/env bash
# ============================================================================
# deploy-container.sh — 日志审计平台容器化部署（测试机执行，root）
# 流程: 生成自签证书 → 构建前端 → 启动 ClickHouse → 校验 init.sql → 全量启动 → 验证
#
# 用法: ./deploy-container.sh
# 前置: 已安装 Docker（docker compose plugin）；本目录含 nginx/clickhouse/api-stub/web-builder
# ============================================================================
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "[1/6] 生成自签 TLS 证书（10 年，测试环境）..."
mkdir -p certs
if [ ! -f certs/audit.crt ] || [ ! -f certs/audit.key ]; then
  openssl req -x509 -nodes -newkey rsa:2048 -days 3650 \
    -keyout certs/audit.key -out certs/audit.crt \
    -subj "/CN=$(hostname -I | awk '{print $1}')" \
    -addext "subjectAltName=IP:$(hostname -I | awk '{print $1}')" \
    -addext "extendedKeyUsage=serverAuth" 2>/dev/null
  echo "[ OK ] 自签证书已生成: certs/audit.crt"
else
  echo "[INFO] 证书已存在，跳过"
fi

echo "[2/6] 构建前端（node 容器）..."
docker compose build web-builder
docker compose run --rm web-builder
ls -la 2>/dev/null || true
# 校验产物
docker run --rm -v audit_web-dist:/d alpine ls /d 2>/dev/null | head -5 || true

echo "[3/6] 启动 ClickHouse 并等待初始化..."
docker compose up -d clickhouse
for i in $(seq 1 30); do
  if docker exec audit-clickhouse clickhouse-client --password "${CLICKHOUSE_PASSWORD:-audit2026}" -q "SELECT 1" >/dev/null 2>&1; then
    echo "[ OK ] ClickHouse 就绪（第 ${i} 次探测）"
    break
  fi
  sleep 2
  [ "$i" = "30" ] && { echo "[ERR] ClickHouse 30 次探测未就绪"; exit 1; }
done

echo "[4/6] 校验 audit 库表（init.sql）..."
docker exec audit-clickhouse clickhouse-client --password "${CLICKHOUSE_PASSWORD:-audit2026}" -q \
  "SELECT name FROM system.tables WHERE database='audit'"
docker exec audit-clickhouse clickhouse-client --password "${CLICKHOUSE_PASSWORD:-audit2026}" -q \
  "SELECT count() FROM system.tables WHERE database='audit' AND name='audit_logs'" | grep -q 1 \
  && echo "[ OK ] audit.audit_logs 表已创建" || { echo "[ERR] audit_logs 表未创建，请检查 init.sql"; exit 1; }

echo "[5/6] 全量启动 nginx + api-stub ..."
docker compose up -d
sleep 3
docker compose ps

echo "[6/6] 部署验证..."
./verify-container.sh || true

echo ""
echo "==================== 完成 ===================="
IP=$(hostname -I | awk '{print $1}')
echo "访问: https://$IP/   （自签证书，浏览器需信任；API 验证: https://$IP/api/v1/auth/me）"
