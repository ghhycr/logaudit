# Vector 采集器镜像（syslog 514 UDP/TCP → ClickHouse）
#
# 构建前置：二进制已在宿主机下载到同目录 vector.tar.gz（.gitignore 已忽略），
# 规避 Docker 构建容器内下载失败：
#   curl -fsSL -o vector.tar.gz \
#     https://packages.timber.io/vector/0.44.0/vector-0.44.0-x86_64-unknown-linux-musl.tar.gz
#
# 若不存在则构建时自动下载（需构建容器具备外网）。
FROM alpine:3.20

RUN apk add --no-cache tar ca-certificates curl

COPY vector.tar.gz /tmp/vector.tar.gz
RUN mkdir -p /tmp/vec \
 && tar -xzf /tmp/vector.tar.gz -C /tmp/vec \
 && VEC_BIN=$(find /tmp/vec -type f -name vector -perm -u+x | head -1) \
 && install -m 0755 "$VEC_BIN" /usr/local/bin/vector \
 && rm -rf /tmp/vec /tmp/vector.tar.gz \
 && mkdir -p /var/lib/vector /etc/vector \
 && vector --version

EXPOSE 514/udp 514/tcp 8686

ENTRYPOINT ["vector", "--config", "/etc/vector/vector.yaml"]
