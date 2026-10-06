# 日志审计平台 - Nginx 站点配置模板（Ubuntu 24.04）
# 说明：
#   - 本文件为模板，占位符 __DOMAIN__ / __TLS_CERT__ / __TLS_KEY__ / __DIST_ROOT__ / __API_UPSTREAM__
#     由 deploy/scripts/install-nginx.sh 用 sed 替换生成，勿直接手改生成的站点配置。
#   - 生成位置：/etc/nginx/sites-available/audit

# HTTP → HTTPS 跳转（等保三级：强制加密传输）
server {
    listen 80;
    listen [::]:80;
    server_name __DOMAIN__;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    http2 on;
    server_name __DOMAIN__;

    # ---------- TLS（等保三级：仅 TLS 1.2+） ----------
    ssl_certificate     __TLS_CERT__;
    ssl_certificate_key __TLS_KEY__;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # ---------- 等保三级安全响应头（入侵防范 / 数据保密） ----------
    add_header X-Frame-Options DENY always;
    add_header X-Content-Type-Options nosniff always;
    add_header Referrer-Policy strict-origin-when-cross-origin always;
    add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # 隐藏 Nginx 版本号（信息泄露防护）
    server_tokens off;

    # ---------- 前端静态资源（Vue3 构建产物） ----------
    root __DIST_ROOT__;
    index index.html;

    # 压缩
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css application/javascript application/json application/xml image/svg+xml;

    # SPA history 路由回退
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 带 hash 的打包产物：长缓存（文件内容变更后文件名变化，缓存安全）
    location /assets/ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }

    # index.html 不缓存（保证新版本发布后立即生效）
    location = /index.html {
        add_header Cache-Control "no-cache, no-store, must-revalidate";
    }

    # ---------- API 反向代理（后端服务 :8080） ----------
    location /api/ {
        proxy_pass __API_UPSTREAM__;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 60s;
        client_max_body_size 10m;
    }

    # ---------- 访问审计（等保三级：安全审计要求留存访问日志） ----------
    access_log /var/log/nginx/audit_access.log combined;
    error_log  /var/log/nginx/audit_error.log warn;
}
