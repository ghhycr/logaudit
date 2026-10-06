# 等保三级安全响应头片段（被 audit.conf 各 context include）
# 注意：nginx add_header 规则——子 context 存在 add_header 时不再继承父级，
# 因此本片段需在 server 与所有 location 中重复 include。
add_header X-Frame-Options DENY always;
add_header X-Content-Type-Options nosniff always;
add_header Referrer-Policy strict-origin-when-cross-origin always;
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'" always;
add_header Strict-Transport-Security "max-age=31536000" always;
add_header X-XSS-Protection "1; mode=block" always;
