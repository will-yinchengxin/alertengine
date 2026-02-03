# 构建阶段
FROM golang:1.23-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git make

# 设置工作目录
WORKDIR /build

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN make build

# 运行阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建应用用户
RUN addgroup -g 1000 alertengine && \
    adduser -D -u 1000 -G alertengine alertengine

# 创建必要的目录
RUN mkdir -p /var/lib/alertengine/rules && \
    mkdir -p /var/log/alertengine && \
    chown -R alertengine:alertengine /var/lib/alertengine /var/log/alertengine

# 复制二进制文件
COPY --from=builder /build/build/alertengine /usr/local/bin/alertengine

# 复制配置文件
COPY config.example.yml /etc/alertengine/config.yml

# 切换到应用用户
USER alertengine

# 暴露端口
EXPOSE 8080 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动命令
ENTRYPOINT ["/usr/local/bin/alertengine"]
CMD ["-config", "/etc/alertengine/config.yml"]
