// Package acl 安置外部服务（LLM、支付、短信等）的防腐适配器。
//
// 准入与方向（细则见 backend/AGENTS.md「外部服务隔离」章节）：
//   - 本包只放外部服务的 adapter；port（抽象接口）不在此定义——
//     用例语义的 port 定义在 application（如 application/auth.TokenIssuer），
//     领域语言的 port 定义在 domain（如 user.Repository）。
//   - adapter 实现 port，按服务分子包（acl/llm/、acl/payment/…），
//     多供应商时子包内再分层。
//   - 外部库的 import 只允许出现在本包及子包内；其类型、错误与响应形状
//     在此转换为业务侧类型与 apperr，不得泄漏进 domain/application。
//   - 自有数据技术（存储与缓存，mysql、redis 等）不走本包，遵循仓储模式，
//     住 persistence/（见 po/mapper 与仓储边界分类）。
package acl
