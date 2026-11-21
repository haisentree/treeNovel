# treeNovel: 小说阅读站 + 爬虫管理后台 (gin + go:embed 单二进制 + sqlite WAL)
#
# 构建 (ACR 个人版拒绝 buildkit 默认 attestation 清单, 必须带 --provenance=false --sbom=false):
#   docker buildx build --provenance=false --sbom=false \
#     -t registry.cn-hangzhou.aliyuncs.com/app_service_haisen/self-tree-novel:1.1 .
#
# 部署: 仓库自带 compose.yaml (echo "ADMIN_PASSWORD=..." > .env && docker compose up -d)
# 数据: 书库 treeNovel.db 打包进镜像 (拉取即用); 爬虫新增数据写在容器可写层,
#       重建容器回退到镜像快照, 需保留增量时重新打包镜像或 docker cp 导出

# ---- 构建阶段 ----
FROM registry.cn-hangzhou.aliyuncs.com/base_service_haisen/mirror-golang:1.27.4-alpine3.24 AS builder

RUN apk add --no-cache git ca-certificates tzdata
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# 全量源码: views/ 与 static/ 经根包 go:embed 打进二进制, 部署只依赖二进制 + db
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/spider ./cmd/spider

# ---- 运行阶段 ----
# 与 self-lemon-movie 同模式: 证书与 zoneinfo 从构建阶段 COPY, 运行阶段零联网
FROM registry.cn-hangzhou.aliyuncs.com/base_service_haisen/mirror-alpine:3.20

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app
COPY --from=builder /out/server /app/server
COPY --from=builder /out/spider /app/spider

# 初始内容库打包进镜像 (94 本书 8214 章, 2026-11-21 快照, WAL 已 checkpoint 合并)
COPY data/treeNovel.db /app/treeNovel.db

ENV TZ=Asia/Shanghai
RUN mkdir -p /app/book

EXPOSE 8083

# busybox wget 探测首页 (爬虫后台任务执行不影响 HTTP 可用性)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q --spider http://127.0.0.1:8083/ || exit 1

ENTRYPOINT ["/app/server"]
# db 在镜像内 /app/treeNovel.db (不在挂载点下, 挂卷不会遮住它);
# 本地电子书目录 /app/book 可选挂载宿主机目录
CMD ["--db", "/app/treeNovel.db", "--bookdir", "/app/book"]
