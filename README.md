# go-clean-template

Go 分层后端脚手架范本：domain / application / infrastructure / presentation / kernel 五层 + gin + GORM(MySQL) + zap + golang-jwt v5。内置注册/登录/刷新/登出（`POST /api/auth/*`）与 `GET /api/users/me` 作为分层规则的活样例，可直接跑通后按模块替换为自有业务。术语见 `CONTEXT.md`，分层约束见 `backend/AGENTS.md`。

## 用此模板起步

1. 全局替换 `go-clean-template` 为你的项目名（module 名、compose 服务与库名、AGENTS 引用）。
2. `cd backend && cp config.example.yaml config.yaml`，填好 `jwt_secret`。
3. 删除不需要的业务模块（auth/user 可留作参照，替换干净后删除）。

## 快速开始（后端）

需要 Docker，然后：

```bash
docker compose up -d --build
curl http://127.0.0.1:8080/healthz   # {"status":"ok"}
```

- 建表由后端启动时 `mysql.Migrate`（GORM AutoMigrate，po 标签为 schema 唯一事实源）自动完成；MySQL 首次建卷无需迁移文件。

本地裸跑 Go 服务（连 compose 提供的 MySQL）：

```bash
cd backend && cp config.example.yaml config.yaml && go run ./cmd/server
```

## 配置

全部配置走工作目录 `config.yaml`（环境变量不再是配置来源）。从 `backend/config.example.yaml` 复制模板；`jwt_secret` 必填，缺失拒绝启动。日志 `log.format` 本地建议 `text`，生产 `json`。
