package logs

import (
	"context"
	"testing"

	w "github.com/erickxeno/logs/writer"
)

func TestCtxAddKVs(t *testing.T) {
	ctx := context.Background()
	ctx = CtxAddKVs(ctx, "hello", "world", 123, 4.56)
	ctx = CtxAddKVs(ctx, "a", "a")
	ctx = CtxAddKVs(ctx, "b", "b")
	ctx = CtxAddKVs(ctx, "c", "c", "c") // ignored
	ctx = CtxAddKVs(ctx, "c", "c", "c", "c")

	// Info 2018-04-24 13:52:33,495 v1(6) ctx_add_kvs_test.go:13 10.2.202.0 - - default - b=b a=a hello=world 123=4.560 bytedance
	V1 := NewCompatLogger(SetWriter(DebugLevel, w.NewConsoleWriter()))
	V1.CtxInfo(ctx, "hel%s", "lo world")

	ctx = context.WithValue(ctx, LogIDCtxKey, "temp_logid")
	ctx = context.WithValue(ctx, spanIDCtxKey, "temp_spanid")
	ctx = context.WithValue(ctx, noticeCtxKey, "temp_notice")
	ctx = context.WithValue(ctx, stackInfoCtxKey, "temp_stack_info")
	ctx = context.WithValue(ctx, DynamicLogLevelKey, "temp_dynamic_log_level")

	V1.CtxInfo(ctx, "hel%s", "lo world")
}
