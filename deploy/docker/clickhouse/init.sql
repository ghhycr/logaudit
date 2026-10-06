-- 日志审计平台 - ClickHouse 初始化（容器首次启动自动执行）
-- 与 设计方案.md 的 audit_logs DDL 保持一致（ZSTD 压缩 / TTL 180 天 / 布隆过滤器）

CREATE DATABASE IF NOT EXISTS audit;

CREATE TABLE IF NOT EXISTS audit.audit_logs
(
    ts           DateTime('Asia/Shanghai') CODEC(DoubleDelta, ZSTD),
    host         LowCardinality(String) CODEC(ZSTD),
    program      LowCardinality(String) CODEC(ZSTD),
    pid          UInt32,
    severity     UInt8,
    facility     UInt8,
    source_ip    IPv4 CODEC(ZSTD),
    user_name    LowCardinality(String) CODEC(ZSTD),
    event_type   LowCardinality(String) CODEC(ZSTD),
    outcome      String CODEC(ZSTD),
    message      String CODEC(ZSTD),
    raw          String CODEC(ZSTD),

    -- 二级索引：消息关键词（ngram 布隆，4 参数：ngram 大小/布隆字节/哈希数/种子）、源 IP
    INDEX idx_msg message TYPE ngrambf_v1(3, 3072, 2, 0) GRANULARITY 4,
    INDEX idx_src source_ip TYPE bloom_filter GRANULARITY 4
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(ts)
ORDER BY (host, ts)
TTL ts + INTERVAL 180 DAY
SETTINGS index_granularity = 8192;

-- 确认（写日志便于排查）
SELECT 'audit.audit_logs created' AS status;
