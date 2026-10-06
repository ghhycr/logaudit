# 前端构建镜像（node 容器内 npm install + build，不污染宿主机/测试机）
FROM node:22-alpine

WORKDIR /build

# 先装依赖（利用层缓存）
COPY web/package.json ./
RUN npm install --no-fund --no-audit

# 再拷源码构建（跳过 vue-tsc 类型检查，加速；类型检查在开发机完成）
COPY web/ ./
RUN npm run build:no-type

# 产物输出到 /build/dist（由 compose 拷贝到 web-dist 卷）
