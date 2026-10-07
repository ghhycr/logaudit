# 日志审计平台 - 授权管理模块（Java / Spring Boot 3 + JDK 17）
#
# 与 Go 后端保持一致的「宿主机编译 + 镜像打包」约定：
#   cd license-service && mvn -B -ntp package -DskipTests     # 产物 target/license-service.jar
#   docker build -t logaudit/license-svc .                    # 本 Dockerfile
#
# 运行环境仅需 JRE，镜像体积小、启动快（JVM 堆 96m~320m）。
FROM eclipse-temurin:17-jre-jammy

WORKDIR /app
ENV TZ=Asia/Shanghai \
    LANG=C.UTF-8

RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

COPY target/license-service.jar /app/license-service.jar

EXPOSE 8091

ENTRYPOINT ["java", "-Xms96m", "-Xmx320m", "-jar", "/app/license-service.jar"]
