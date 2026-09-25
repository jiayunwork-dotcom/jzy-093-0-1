# syntax=docker/dockerfile:1

# ---- 构建与测试阶段：测试在镜像构建过程中真实执行，失败则构建失败 ----
FROM golang:1.22-bookworm AS build
WORKDIR /src

# 先拉依赖（本项目零外部依赖，保留缓存层写法便于扩展）
COPY go.mod ./
RUN go mod download

# 源码与测试一起进镜像
COPY . .

# vet + 全套自动化测试必须在容器里跑通
RUN go vet ./... && go test -race -count=1 ./...

# 静态构建
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/roserver ./cmd/server

# ---- 运行阶段：非 root、固定暴露 8080 ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/roserver /roserver
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/roserver"]
