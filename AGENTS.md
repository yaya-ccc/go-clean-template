# AGENTS.md — go-clean-template 项目规范

本文件与 `backend/AGENTS.md` 是项目的最终约束，后续开发人员与 AI 助手必须遵守。产品域术语以根目录 `CONTEXT.md` 为准，后端实现术语以 `backend/CONTEXT.md` 为准。设计过程的历史决策记录（ADR）已删除，其最终结论均已收拢进 AGENTS 与 CONTEXT；遇到与本文冲突的旧习惯以本文为准。

## 目录结构

```
go-clean-template/
├── CONTEXT.md                  # 全项目唯一术语表（只定义概念，不记录实现细节）
├── docs/
│   └── 日志设计.md
├── backend/
│   ├── AGENTS.md               # 后端最终约束（分层职责、错误、校验、持久化、表现层规则）
│   ├── docs/
│   │   └── api.md              # 对外 HTTP 接口文档
│   ├── cmd/server/main.go      # 唯一组装根
│   ├── config.example.yaml     # 配置模板；真实 config.yaml 已 gitignore
│   └── internal/
│       ├── domain/             # 领域层：模型、业务规则、仓储契约；按模块分包
│       ├── application/        # 应用层：用例编排；按模块分包
│       ├── infrastructure/     # 基础设施层：mysql 仓储 + po 持久化模型
│       ├── presentation/       # 表现层：api/（业务模块包）+ response + middleware + router（协议公共件）
│       └── kernel/             # 横切组件：apperr / applog / config
```

- 技术栈：Go 1.26 + gin + GORM(MySQL) + zap + golang-jwt v5；新增依赖前先问「stdlib/既有依赖能不能解决」。
- 写后端代码前必读 `backend/AGENTS.md`；改 handler/错误码必须同步 `backend/docs/api.md` 的错误码表与接口描述。

## 工作流约定

- 提交：实现与验证完成后自动创建 commit；只暂存本次需求涉及的文件，禁止 `git add .`/`git add -A`；标题 `<type>(<scope>): <中文摘要>`，正文中文有序列表。`.idea/`、`config.yaml`、计划文档不入库。
- 验证：非平凡行为改动至少保留一个可能失败且有意义的检查；改完跑 `gofmt` 与受影响包测试，跨包契约改动跑 `go test ./...`。
- 新增 Go 导出类型/函数/方法必须写中文文档注释（说明原因、约束、不变量，不复述代码）。
- 新增横切组件 = `internal/kernel/` 下建子包，准入条件是零业务 import 依赖。
- 新决策满足「难逆 + 无上下文会困惑 + 真实权衡」时，直接更新 AGENTS 相应章节，不再另设决策记录目录。
