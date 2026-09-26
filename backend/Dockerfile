# 后端镜像：多阶段构建，产物为纯 Go 静态二进制。
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3
COPY --from=build /out/server /server
# 配置经挂载注入工作目录 /srv 的 config.yaml；容器内必须绑定 0.0.0.0。
WORKDIR /srv
EXPOSE 8080
ENTRYPOINT ["/server"]
