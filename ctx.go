package logs

import (
	"bytes"
	"context"
	"sync"

	"github.com/erickxeno/logs/writer"
)

// 定义自定义类型作为context key
type contextKey string

const (
	LogIDCtxKey        = contextKey("K_LOGID")
	kvCtxKey           = contextKey("K_KVs")
	spanIDCtxKey       = contextKey("K_SPANID")
	noticeCtxKey       = contextKey("K_NOTICE")
	stackInfoCtxKey    = contextKey("K_STACK_INFO")
	DynamicLogLevelKey = contextKey("K_DYNAMIC_LOG_LEVEL")
)

func GetNewLogIDCtxKey(newKey string) contextKey {
	return contextKey(newKey)
}

var (
	convertCtxKVListToStr = false
	spaceBytes            = []byte{' '}
)

type StackInfo byte

type ctxKVs struct {
	kvs []interface{}
	pre *ctxKVs
}

const (
	NoPrint StackInfo = iota
	CurrGoroutine
	AllGoroutines
)

// CtxAddKVs works like logs.CtxAddKVs in logs 1.0
func CtxAddKVs(ctx context.Context, kvs ...interface{}) context.Context {
	if len(kvs) == 0 || (len(kvs)&1 == 1) {
		return ctx
	}

	kvList := make([]interface{}, 0, len(kvs))
	kvList = append(kvList, kvs...)

	return context.WithValue(ctx, kvCtxKey, &ctxKVs{
		kvs: kvList,
		pre: getKVs(ctx),
	})
}

func getKVs(ctx context.Context) *ctxKVs {
	if ctx == nil {
		return nil
	}
	i := ctx.Value(kvCtxKey)
	if i == nil {
		return nil
	}
	if kvs, ok := i.(*ctxKVs); ok {
		return kvs
	}
	return nil
}

func GetAllKVs(ctx context.Context) []interface{} {
	if ctx == nil {
		return nil
	}
	kvs := getKVs(ctx)
	if kvs == nil {
		return nil
	}

	var result []interface{}
	recursiveAllKVs(&result, kvs, 0)
	return result
}

// to keep FIFO order
func recursiveAllKVs(result *[]interface{}, kvs *ctxKVs, total int) {
	if kvs == nil {
		*result = make([]interface{}, 0, total)
		return
	}
	recursiveAllKVs(result, kvs.pre, total+len(kvs.kvs))
	*result = append(*result, kvs.kvs...)
}

func GetAllKVsStr(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	kvs := getKVs(ctx)
	if kvs == nil {
		return ""
	}

	var result []interface{}
	recursiveAllKVs(&result, kvs, 0)

	var buf bytes.Buffer
	for i := 0; i+1 < len(result); i += 2 {
		kv := writer.NewOmniKeyValue(result[i], result[i+1])
		if i != 0 {
			buf.Write(spaceBytes)
		}
		buf.WriteString(kv.String())
	}
	return buf.String()
}

type NoticeKVs struct {
	kvs []interface{}
	sync.Mutex
}

func (l *NoticeKVs) PushNotice(k, v interface{}) {
	l.Lock()
	l.kvs = append(l.kvs, k, v)
	l.Unlock()
}

func (l *NoticeKVs) KVs() []interface{} {
	l.Lock()
	kvs := l.kvs
	l.Unlock()
	return kvs
}

func newNoticeKVs() *NoticeKVs {
	return &NoticeKVs{
		kvs: make([]interface{}, 0, 16),
	}
}

func NewNoticeCtx(ctx context.Context) context.Context {
	ntc := newNoticeKVs()
	return context.WithValue(ctx, noticeCtxKey, ntc)
}

func GetNotice(ctx context.Context) *NoticeKVs {
	i := ctx.Value(noticeCtxKey)
	if ntc, ok := i.(*NoticeKVs); ok {
		return ntc
	}
	return nil
}

func CtxPushNotice(ctx context.Context, k, v interface{}) {
	ntc := GetNotice(ctx)
	if ntc == nil {
		return
	}
	ntc.PushNotice(k, v)
}

// CtxStackInfo marks whether to print the stack in the current log.
func CtxStackInfo(ctx context.Context, stackInfo StackInfo) context.Context {
	return context.WithValue(ctx, stackInfoCtxKey, stackInfo)
}

func logIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return "-"
	}
	val := ctx.Value(LogIDCtxKey)
	if val != nil {
		logID := val.(string)
		return logID
	}
	return "-"
}

/*
Span:  一个有时间跨度的事件，例如一次服务调用，一个函数执行
Event: 一个事件，没有时间跨度，例如一次panic，一个订单生成
Metric: 一个带多维tag的数值，例如一个消息体的大小
Trace: 一个跨越了多个分布式服务的完整请求链路
Transaction(后续简称Txn): Trace的组成单元，记录Trace在单个服务节点上的多个Span/Event/Metric对象构成的树形结构，

	Txn不用使用方显式地创建和关闭，而是由SDK自动将同一个请求上下文中的Span/Event/Metric对象打包在一个Txn中
*/
func spanIDFromContext(ctx context.Context) uint64 {
	if ctx == nil {
		return 0
	}
	val := ctx.Value(spanIDCtxKey)
	if val != nil {
		spanID, valid := val.(uint64)
		if valid {
			return spanID
		}
	}
	return 0
}
