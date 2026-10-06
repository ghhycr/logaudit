#!/usr/bin/env bash
# ============================================================================
# deploy-frontend.sh — 部署前端构建产物到 Nginx 站点目录
# 支持: 本机拷贝（cp） 或 远程服务器（rsync + ssh）
#
# 用法:
#   # 本机部署（产物已在目标机）
#   ./deploy-frontend.sh --src /path/to/web/dist
#
#   # 远程部署（开发机 → 服务器，需服务器已跑 install-nginx.sh）
#   ./deploy-frontend.sh --src ./web/dist --host user@server --dest /opt/audit/web/dist
#
#   # 默认值
#   ./deploy-frontend.sh                          # src=./web/dist dest=/opt/audit/web/dist
# ============================================================================
set -euo pipefail

SRC="${SRC:-$(pwd)/web/dist}"
DEST="${DEST:-/opt/audit/web/dist}"
HOST="${HOST:-}"
RELOAD_NGINX="${RELOAD_NGINX:-1}"

usage() {
  cat <<EOF
用法: $0 [选项]
  --src PATH        前端构建产物目录（含 index.html；默认 ./web/dist）
  --dest PATH       服务器目标目录（默认 /opt/audit/web/dist）
  --host user@server  远程服务器（缺省=本机部署）
  --no-reload       部署后不重载 nginx（默认自动重载）
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --src)      SRC="$2"; shift 2 ;;
    --dest)     DEST="$2"; shift 2 ;;
    --host)     HOST="$2"; shift 2 ;;
    --no-reload) RELOAD_NGINX=0; shift ;;
    -h|--help)  usage; exit 0 ;;
    *) echo "[ERR] 未知参数: $1"; usage; exit 1 ;;
  esac
done

# ---------- 校验源产物 ----------
[ -d "$SRC" ] || { echo "[ERR] 源目录不存在: $SRC（请先执行 cd web && npm run build）"; exit 1; }
[ -f "$SRC/index.html" ] || { echo "[ERR] $SRC/index.html 缺失，不是有效的前端构建产物"; exit 1; }
echo "[INFO] 源产物: $SRC（index.html 已确认）"

# ---------- 备份旧版本 ----------
backup_remote() {
  local dest="$1"
  if ssh -o ConnectTimeout=5 -o BatchMode=yes "$HOST" "[ -d $dest ] && [ -n \"\$(ls -A $dest 2>/dev/null)\" ]" 2>/dev/null; then
    local ts
    ts=$(date +%Y%m%d%H%M%S)
    ssh "$HOST" "cp -a $dest ${dest}.bak-$ts && echo '[ OK ] 旧版本已备份为 ${dest}.bak-'$ts"
  fi
}

if [ -n "$HOST" ]; then
  # ---------- 远程部署（rsync） ----------
  echo "[INFO] 远程部署: $HOST:$DEST"
  command -v rsync >/dev/null || { echo "[ERR] 本机未安装 rsync，请先安装（Ubuntu: apt install rsync）"; exit 1; }
  ssh -o ConnectTimeout=5 -o BatchMode=yes "$HOST" "true" 2>/dev/null \
    || { echo "[ERR] 无法 SSH 连接 $HOST（请确认免密登录已配置）"; exit 1; }

  backup_remote "$DEST"

  rsync -az --delete -e ssh "$SRC/" "$HOST:$DEST/"
  ssh "$HOST" "chown -R www-data:www-data $DEST 2>/dev/null || true"
  echo "[ OK ] 前端产物已同步到 $HOST:$DEST"

  if [ "$RELOAD_NGINX" = "1" ]; then
    ssh "$HOST" "nginx -t && systemctl reload nginx" \
      || { echo "[ERR] 远程 nginx 重载失败"; exit 1; }
    echo "[ OK ] 远程 nginx 已重载"
  fi
else
  # ---------- 本机部署（cp） ----------
  echo "[INFO] 本机部署到 $DEST"
  [ "$(id -u)" -eq 0 ] || { echo "[ERR] 本机部署到 $DEST 需要 root 权限，请 sudo 运行"; exit 1; }
  [ -d "$DEST" ] || { echo "[ERR] 目标目录不存在: $DEST（请先运行 install-nginx.sh）"; exit 1; }

  if [ -n "$(ls -A "$DEST" 2>/dev/null)" ]; then
    ts=$(date +%Y%m%d%H%M%S)
    cp -a "$DEST" "${DEST}.bak-$ts"
    echo "[ OK ] 旧版本已备份为 ${DEST}.bak-$ts"
  fi

  cp -a "$SRC"/. "$DEST"/
  chown -R www-data:www-data "$DEST"
  echo "[ OK ] 前端产物已部署到 $DEST"

  if [ "$RELOAD_NGINX" = "1" ]; then
    nginx -t && systemctl reload nginx
    echo "[ OK ] nginx 已重载"
  fi
fi

echo ""
echo "==================== 完成 ===================="
echo "访问验证: curl -k https://$(hostname -f)/  或运行 verify.sh 全面检查"
