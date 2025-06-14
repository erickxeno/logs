package logs

import (
	"context"
	"fmt"
	"os"
	"sync"
)

const (
	compatCallDepthOffset = 1
)

var (
	compatPadding = []byte("")
)

func (l *Log) fastSPrintf(format string, args ...interface{}) *Log {
	if l == nil {
		return nil
	}
	inVerb := false
	lastWroteAt := 0
	verbStartAt := 0
	verbCount := 0
	for i := 0; i < len(format); i++ {
		char := format[i]
		if char != '%' {
			if !inVerb {
				continue
			}
			// fmt verbs have to end with a single alphabet
			if !('a' <= char && char <= 'z') && !('A' <= char && char <= 'Z') {
				continue
			}
			inVerb = false
			l.Str(format[lastWroteAt:verbStartAt]).trimPadding()

			var arg interface{}
			if verbCount > len(args)-1 {
				arg = "empty?"
			} else {
				arg = args[verbCount]
			}
			switch a := arg.(type) {
			case Lazier:
				arg = a()
			default:
			}
			switch char {
			case 'd':
				switch v := arg.(type) {
				case int64:
					l.Int(int(v)).trimPadding()
				case int32:
					l.Int(int(v)).trimPadding()
				case int:
					l.Int(v).trimPadding()
				default:
					l.Str(fmt.Sprintf("%d", v)).trimPadding()
				}
			case 'f':
				switch v := arg.(type) {
				case float64:
					l.Float(v).trimPadding()
				case float32:
					l.Float(float64(v)).trimPadding()
				default:
					l.Str(fmt.Sprintf("%f", v)).trimPadding()
				}
			case 't':
				switch v := arg.(type) {
				case bool:
					l.Bool(v).trimPadding()
				default:
					l.Str(fmt.Sprintf("%t", v)).trimPadding()
				}
			case 's':
				switch v := arg.(type) {
				case string:
					l.Str(v).trimPadding()
				case fmt.Stringer:
					l.Str(v.String()).trimPadding()
				case error:
					l.Str(v.Error()).trimPadding()
				default:
					l.Str(fmt.Sprintf("%s", v)).trimPadding()
				}
			case 'x':
				l.Str(fmt.Sprintf("%x", arg)).trimPadding()
			case 'v':
				if i >= 1 && format[i-1] == '+' {
					l.Str(fmt.Sprintf("%+v", arg)).trimPadding()
				} else {
					l.Obj(arg).trimPadding()
				}
			case 'p':
				l.Str(fmt.Sprintf("%p", arg)).trimPadding()
			default:
				l.Obj(arg).trimPadding()
			}
			verbCount++
			lastWroteAt = i + 1
			continue
		}
		// handle '%%'
		if i < len(format)-1 && format[i+1] == '%' {
			l.Str(format[lastWroteAt:i]).trimPadding()
			l.Str("%").trimPadding()
			lastWroteAt = i + 2
			i++
			inVerb = false
			continue
		}

		inVerb = true
		verbStartAt = i
	}
	if lastWroteAt < len(format) {
		l.Str(format[lastWroteAt:]).trimPadding()
	}
	return l
}

// CompatLogger CLogger does not support std string format originally,
// CompatLogger creates a simple fast approach fmt.Sprintf,
// map %s to Log.Str, %f to Log.Float, %v to json.Marshal and ignore any prefix on verbs.
// it is not only able to boost the format performance but also keep the compatibility.
type CompatLogger struct {
	v1                 *CLogger
	callDepthOffset    int
	enableDynamicLevel bool
}

// NewCompatLogger creates a compatible logger CompatLogger,
// it supports Info, CtxInfo, CtxInfosf, CtxInfoKVs and other level methods.
func NewCompatLogger(ops ...Option) *CompatLogger {
	v1 := NewCLogger(ops...)
	return &CompatLogger{
		v1:                 v1,
		callDepthOffset:    compatCallDepthOffset,
		enableDynamicLevel: v1.compatLoggerDynamicLevel,
	}
}

// NewCompatLoggerFrom creates a compatible logger CompatLogger from a CLogger.
func NewCompatLoggerFrom(logger *CLogger, ops ...CompatibleLoggerOption) *CompatLogger {
	l := &CompatLogger{
		v1:                 logger,
		callDepthOffset:    compatCallDepthOffset,
		enableDynamicLevel: logger.compatLoggerDynamicLevel,
	}

	for _, op := range ops {
		op(l)
	}
	return l
}

func (l *CompatLogger) newLog(level Level, ctx context.Context) *Log {
	if l == nil {
		return nil
	}

	var log *Log
	if ctx == nil {
		log = l.v1.prefix(l.v1.newLog(level))
	} else {
		if l.enableDynamicLevel {
			log = l.v1.prefix(l.v1.newLog(level, WithCtx(ctx)))
		} else {
			log = l.v1.prefix(l.v1.newLog(level)).With(ctx)
		}
	}
	return log.CallDepth(l.callDepthOffset)
}

func (l *CompatLogger) Fatal(format string, v ...interface{}) {
	l.newLog(FatalLevel, nil).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) CtxFatal(ctx context.Context, format string, v ...interface{}) {
	l.newLog(FatalLevel, ctx).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) Error(format string, v ...interface{}) {
	l.newLog(ErrorLevel, nil).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) CtxError(ctx context.Context, format string, v ...interface{}) {
	l.newLog(ErrorLevel, ctx).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) Warn(format string, v ...interface{}) {
	l.newLog(WarnLevel, nil).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) CtxWarn(ctx context.Context, format string, v ...interface{}) {
	l.newLog(WarnLevel, ctx).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) Notice(format string, v ...interface{}) {
	l.newLog(NoticeLevel, nil).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) CtxNotice(ctx context.Context, format string, v ...interface{}) {
	l.newLog(NoticeLevel, ctx).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) Info(format string, v ...interface{}) {
	l.newLog(InfoLevel, nil).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) CtxInfo(ctx context.Context, format string, v ...interface{}) {
	l.newLog(InfoLevel, ctx).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) Debug(format string, v ...interface{}) {
	l.newLog(DebugLevel, nil).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) CtxDebug(ctx context.Context, format string, v ...interface{}) {
	l.newLog(DebugLevel, ctx).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) Trace(format string, v ...interface{}) {
	l.newLog(TraceLevel, nil).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) CtxTrace(ctx context.Context, format string, v ...interface{}) {
	l.newLog(TraceLevel, ctx).fastSPrintf(format, v...).Emit()
}

func (l *CompatLogger) CtxFatalsf(ctx context.Context, format string, v ...string) {
	strSlice := make([]interface{}, 0, len(v))
	for _, s := range v {
		strSlice = append(strSlice, s)
	}

	l.newLog(FatalLevel, ctx).fastSPrintf(format, strSlice...).Emit()
}

func (l *CompatLogger) CtxErrorsf(ctx context.Context, format string, v ...string) {
	strSlice := make([]interface{}, 0, len(v))
	for _, s := range v {
		strSlice = append(strSlice, s)
	}
	l.newLog(ErrorLevel, ctx).fastSPrintf(format, strSlice...).Emit()
}

func (l *CompatLogger) CtxWarnsf(ctx context.Context, format string, v ...string) {
	strSlice := make([]interface{}, 0, len(v))
	for _, s := range v {
		strSlice = append(strSlice, s)
	}
	l.newLog(WarnLevel, ctx).fastSPrintf(format, strSlice...).Emit()
}

func (l *CompatLogger) CtxNoticesf(ctx context.Context, format string, v ...string) {
	strSlice := make([]interface{}, 0, len(v))
	for _, s := range v {
		strSlice = append(strSlice, s)
	}
	l.newLog(NoticeLevel, ctx).fastSPrintf(format, strSlice...).Emit()
}

func (l *CompatLogger) CtxInfosf(ctx context.Context, format string, v ...string) {
	strSlice := make([]interface{}, 0, len(v))
	for _, s := range v {
		strSlice = append(strSlice, s)
	}
	l.newLog(InfoLevel, ctx).fastSPrintf(format, strSlice...).Emit()
}
func (l *CompatLogger) CtxDebugsf(ctx context.Context, format string, v ...string) {
	strSlice := make([]interface{}, 0, len(v))
	for _, s := range v {
		strSlice = append(strSlice, s)
	}
	l.newLog(DebugLevel, ctx).fastSPrintf(format, strSlice...).Emit()
}

func (l *CompatLogger) CtxTracesf(ctx context.Context, format string, v ...string) {
	strSlice := make([]interface{}, 0, len(v))
	for _, s := range v {
		strSlice = append(strSlice, s)
	}
	l.newLog(TraceLevel, ctx).fastSPrintf(format, strSlice...).Emit()
}

func (l *CompatLogger) CtxFatalKVs(ctx context.Context, kvs ...interface{}) {
	l.newLog(FatalLevel, ctx).KVs(kvs...).Emit()
}

func (l *CompatLogger) CtxErrorKVs(ctx context.Context, kvs ...interface{}) {
	l.newLog(ErrorLevel, ctx).KVs(kvs...).Emit()
}

func (l *CompatLogger) CtxWarnKVs(ctx context.Context, kvs ...interface{}) {
	l.newLog(WarnLevel, ctx).KVs(kvs...).Emit()
}

func (l *CompatLogger) CtxNoticeKVs(ctx context.Context, kvs ...interface{}) {
	l.newLog(NoticeLevel, ctx).KVs(kvs...).Emit()
}

func (l *CompatLogger) CtxInfoKVs(ctx context.Context, kvs ...interface{}) {
	l.newLog(InfoLevel, ctx).KVs(kvs...).Emit()
}

func (l *CompatLogger) CtxDebugKVs(ctx context.Context, kvs ...interface{}) {
	l.newLog(DebugLevel, ctx).KVs(kvs...).Emit()
}

func (l *CompatLogger) CtxTraceKVs(ctx context.Context, kvs ...interface{}) {
	l.newLog(TraceLevel, ctx).KVs(kvs...).Emit()
}

func (l *CompatLogger) CtxFlushNotice(ctx context.Context) {
	var kvs []interface{}
	ntc := GetNotice(ctx)
	if ntc != nil {
		kvs = ntc.KVs()
	}
	l.newLog(NoticeLevel, ctx).KVs(kvs...).Emit()
}

// PrintStack prints the stacks in Info level.
// If printAllGoroutines is true, it prints the stacks of all goroutines.
// Otherwise, it just prints the current goroutine's stack.
func (l *CompatLogger) PrintStack(printAllGoroutines bool) {
	ctx := context.Background()
	if printAllGoroutines {
		ctx = CtxStackInfo(ctx, AllGoroutines)
	} else {
		ctx = CtxStackInfo(ctx, CurrGoroutine)
	}
	l.CtxInfo(ctx, "")
}

// Flush blocks logger and flush all buffered logs.
func (l *CompatLogger) Flush() {
	if l == nil {
		return
	}
	err := l.v1.Flush()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "logs client flush error: %s\n", err)
	}
}

// Close flushes logger and graceful exit.
func (l *CompatLogger) Close() error {
	if l == nil {
		return nil
	}
	return l.v1.Close()
}

// Stop graceful exit with no error returned.
func (l *CompatLogger) Stop() {
	if l == nil {
		return
	}
	err := l.v1.Close()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "logs client stop error: %s\n", err)
	}
}

func (l *Log) setKVToMap(m interface{}, kvs ...interface{}) *Log {
	if l == nil {
		return nil
	}
	if m != nil {
		if syncMap, ok := m.(*sync.Map); ok {
			syncMap.Range(func(key, value interface{}) bool {
				l = l.KV(key, value)
				return true
			})
		}
	}

	for i := 0; i+1 < len(kvs); i += 2 {
		k := kvs[i]
		v := kvs[i+1]
		l = l.KV(k, v)
	}
	return l
}
