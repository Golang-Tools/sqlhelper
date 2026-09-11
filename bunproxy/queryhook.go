package bunproxy

import (
	"context"
	"database/sql"
	"errors"
	"time"

	log "github.com/Golang-Tools/loggerhelper/v4"
	"github.com/uptrace/bun"
)

// queryLogHook 基于slog的查询日志钩子
// 默认只输出脱敏后的SQL模板,不输出参数值,避免敏感数据落入日志
type queryLogHook struct {
	//withArgs 为true时额外输出参数值
	withArgs bool
}

// newQueryLogHook 创建一个查询日志钩子
func newQueryLogHook(withArgs bool) *queryLogHook {
	return &queryLogHook{withArgs: withArgs}
}

// BeforeQuery 实现bun.QueryHook,这里不做任何处理
func (h *queryLogHook) BeforeQuery(ctx context.Context, evt *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery 实现bun.QueryHook,输出查询耗时与脱敏后的SQL
func (h *queryLogHook) AfterQuery(ctx context.Context, evt *bun.QueryEvent) {
	if evt == nil {
		return
	}
	cost := time.Since(evt.StartTime)
	fields := map[string]interface{}{
		"module":    "bun-proxy",
		"operation": evt.Operation(),
		"cost_ms":   float64(cost.Microseconds()) / 1000,
		"sql":       SanitizeSQL(evt.QueryTemplate),
	}
	if h.withArgs && len(evt.QueryArgs) > 0 {
		fields["args"] = evt.QueryArgs
	}
	switch {
	case evt.Err == nil, errors.Is(evt.Err, sql.ErrNoRows):
		log.Debug("bun query done", fields)
	case errors.Is(evt.Err, context.Canceled),
		errors.Is(evt.Err, context.DeadlineExceeded):
		fields["err"] = evt.Err.Error()
		log.Warn("bun query canceled", fields)
	default:
		fields["err"] = evt.Err.Error()
		log.Error("bun query failed", fields)
	}
}
