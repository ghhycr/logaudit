# LogAudit API（M2 认证服务）容器镜像
# 构建: docker build -t logaudit/api .
# 说明: Go 后端纯静态编译（CGO_ENABLED=0），可直接运行于 scratch 空镜像，
#       无需拉取任何基础镜像（避开慢速镜像源）。
FROM scratch

COPY audit-api /audit-api
EXPOSE 8080
ENTRYPOINT ["/audit-api"]
