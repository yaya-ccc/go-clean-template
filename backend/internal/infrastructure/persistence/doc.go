// Package persistence 安置自有数据技术（存储与缓存）的仓储实现。
//
// 准入与方向（细则见 backend/AGENTS.md「持久化」与「外部服务隔离」章节）：
//   - 本包按技术分子包（mysql/、redis/…），子包内遵循仓储模式：po/ 表形状、
//     mapper.go 转换、驱动错误边界分类。
//   - 缓存用途的 redis 同样住本包：存储与缓存同属自有数据面，成员规则不按
//     用途二次切分。
//   - 外部服务（LLM、支付、OSS 对象存储等）不走本包，遵循 port + adapter，
//     住 infrastructure/acl/——即使它也存数据，它是别人的系统。
package persistence
