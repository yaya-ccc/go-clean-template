// Package applog 提供全服务统一的过程日志：zap 全局 logger、请求级 kv 过程数据收集器。
// 设计见 docs/日志设计.md；applog 无业务语义，应用层与领域层均可依赖。
package applog

import (
	"context"
	"os"
	"sync"
	"time"

	"go-clean-template/backend/internal/kernel/apperr"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// collectorKey 是 context 中存放过程数据收集器的键类型。
type collectorKey struct{}

// reservedKeys 是中间件自动写入的保留键，业务键不得占用。
var reservedKeys = map[string]struct{}{
	"request_id": {}, "method": {}, "path": {}, "status": {},
	"latency_ms": {}, "client_ip": {}, "user_id": {}, "error": {},
	"outcome": {}, "route": {}, "public_error_code": {}, "error_chain": {},
	"cause_type": {}, "cause": {},
}

// KVer 是过程数据值的受限类型集标记接口；只有本包提供的构造器能实现它。
type KVer interface{ logKV() any }

type strKV string
type intKV int64
type floatKV float64
type boolKV bool
type timeKV time.Time

// errKV 包装错误为受限类型集成员；接口类型不能直接做方法接收者。
type errKV struct{ err error }

func (v strKV) logKV() any   { return string(v) }
func (v intKV) logKV() any   { return int64(v) }
func (v floatKV) logKV() any { return float64(v) }
func (v boolKV) logKV() any  { return bool(v) }
func (v timeKV) logKV() any  { return time.Time(v).Format(time.RFC3339) }
func (v errKV) logKV() any {
	if v.err == nil {
		return ""
	}
	// code 与 op 分开输出，不调用底层 Error() 拼接，也不合并成单字符串：
	// 日志侧按字段过滤/聚合（如按 code 统计）时无需再解析。
	if e, ok := apperr.First(v.err); ok {
		return map[string]any{"code": string(e.Code), "op": e.Message}
	}
	return "non-apperr error"
}

// String 构造字符串过程值。
func String(s string) KVer { return strKV(s) }

// Int64 构造整数过程值。
func Int64(n int64) KVer { return intKV(n) }

// Float64 构造浮点过程值。
func Float64(f float64) KVer { return floatKV(f) }

// Bool 构造布尔过程值。
func Bool(b bool) KVer { return boolKV(b) }

// Time 构造时间过程值，输出统一为 RFC3339 字符串。
func Time(t time.Time) KVer { return timeKV(t) }

// Err 构造错误过程值；nil 输出空串。
func Err(err error) KVer { return errKV{err: err} }

// collector 累积单个请求的过程数据；并发安全（同一请求内可能并发 Add）。
type collector struct {
	mu sync.Mutex
	kv map[string]any
}

// NewCollector 返回携带过程数据收集器的子 context；请求结束由中间件统一输出。
func NewCollector(ctx context.Context) context.Context {
	return context.WithValue(ctx, collectorKey{}, &collector{kv: map[string]any{}})
}

// Add 向当前请求的过程数据追加一条 kv：键须为小写 snake_case 且不占用保留键；
// 命中保留键或键非法时丢弃并记 debug（规格：静默丢弃并 debug 记录）。
func Add(ctx context.Context, key string, value KVer) {
	if value == nil {
		return
	}
	if _, reserved := reservedKeys[key]; reserved {
		L().Debug("applog: 保留键被业务占用，已丢弃", zap.String("key", key))
		return
	}
	if !validKey(key) {
		L().Debug("applog: 键不符合 snake_case，已丢弃", zap.String("key", key))
		return
	}
	if c, ok := ctx.Value(collectorKey{}).(*collector); ok && c != nil {
		c.mu.Lock()
		c.kv[key] = value.logKV()
		c.mu.Unlock()
	}
}

// validKey 校验键为小写 snake_case（[a-z0-9_]+ 且首字符非数字非下划线）且非保留键。
func validKey(key string) bool {
	if _, reserved := reservedKeys[key]; reserved {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		case c == '_' && i > 0:
		default:
			return false
		}
	}
	return len(key) > 0
}

// Fields 返回当前请求已收集的过程数据快照；无收集器时返回 nil。
// 供过程日志中间件在请求结束时合并输出；业务代码只应使用 Add。
func Fields(ctx context.Context) map[string]any {
	c, ok := ctx.Value(collectorKey{}).(*collector)
	if !ok || c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]any, len(c.kv))
	for k, v := range c.kv {
		out[k] = v
	}
	return out
}

// Config 是日志初始化参数，由 config.yaml 的 log 段映射。
type Config struct {
	Format string // text | json，缺省 json
	Level  string // debug | info | warn | error，缺省 info
}

// global 是全局 logger；Init 之前为零值可用的 NOP logger，避免强制初始化顺序。
var global = zap.NewNop()

// Init 按配置初始化全局 logger，输出到 stdout；进程生命周期调用一次。
func Init(cfg Config) {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.RFC3339TimeEncoder
	encCfg.EncodeLevel = zapcore.LowercaseLevelEncoder

	var enc zapcore.Encoder = zapcore.NewJSONEncoder(encCfg)
	if cfg.Format == "text" {
		enc = zapcore.NewConsoleEncoder(encCfg)
	}
	global = zap.New(zapcore.NewCore(enc, zapcore.Lock(os.Stdout), parseLevel(cfg.Level)))
}

// L 返回全局 logger；未 Init 时返回 NOP logger（测试零配置）。
func L() *zap.Logger { return global }

// parseLevel 解析级别名；未识别或为空时回落 info。
func parseLevel(s string) zapcore.Level {
	var lv zapcore.Level
	if err := lv.UnmarshalText([]byte(s)); err != nil {
		return zapcore.InfoLevel
	}
	return lv
}
