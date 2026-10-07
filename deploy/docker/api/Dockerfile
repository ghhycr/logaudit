# LogAudit 后端 API 容器镜像（Go 静态编译产物）
# 构建前请在宿主机执行：
#   cd backend && CGO_ENABLED=0 go build -o ../deploy/docker/api/audit-api .
FROM scratch

COPY audit-api /audit-api
EXPOSE 8080
ENTRYPOINT ["/audit-api"]
