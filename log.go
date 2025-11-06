package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	osTime "time"
	"unsafe"

	"golang.org/x/time/rate"

	"github.com/erickxeno/logs/writer"
)

/*
Why defaultCallDepth is 2 and preparedCallDepthOffset is 4 here?

When we use the following chaining methods to print logs:
logger.Info().Emit()
The stack frames should look like the following.
(*Log).Emit() -> (*prefixedLog).Location.func1 -> runtime.Caller()

	2                           1                       0

If we set callDepth = 0, we will get the file and line number information of the runtime.Caller(), which is prefix.go: 56.
If we set callDepth = 1, we will get the file and line number information of the anonymous function, which is log.go: 289
If we set callDepth = 2, we will get the file and line number information of the Emit() function.

preparedCallDepthOffset is used in line.go: 22. When we use Line(), the call graph should be look like below.
(*Log).Emit() ->  (*prefixedLog).Location.func1 -> (*Line).load -> sync.(*Once).Do() -> sync.(*Once).doSlow()-> (*Line).load.func1() -> runtime.Caller()

	6                           5                        4                 3                 2                        1                     0

To get the location information of Emit(), we need to set callDepth = 6, which is 2 + 4. That's why preparedCallDepthOffset is 4.
*/
const (
	version                 = "v1(0)"
	defaultCallDepth        = 2
	preparedCallDepthOffset = 4
	_TracebackMaxFrames     = 20
)

// Level defines log levels
type Level int32

const (
	// TraceLevel defines Trace Level.
	TraceLevel Level = iota
	// DebugLevel defines Debug Level.
	DebugLevel
	// InfoLevel defines Info Level.
	InfoLevel
	// NoticeLevel defines Notice Level.
	NoticeLevel
	// WarnLevel defines Warn Level.
	WarnLevel
	// ErrorLevel defines Error Level.
	ErrorLevel
	// FatalLevel defines Fatal Level.
	FatalLevel
)

// String stringify Level.
func (l Level) String() string {
	switch l {
	case TraceLevel:
		return "Trace"
	case DebugLevel:
		return "Debug"
	case InfoLevel:
		return "Info"
	case NoticeLevel:
		return "Notice"
	case WarnLevel:
		return "Warn"
	case ErrorLevel:
		return "Error"
	case FatalLevel:
		return "Fatal"
	}
	return "?"
}

// KVPosition defines the position of the kv list
type KVPosition int32

const (
	BeforeMsg KVPosition = iota
	AfterMsg
)

const (
	spaceByte = ' '
)

var logPool = &sync.Pool{
	New: func() interface{} {
		return &Log{
			buf:             make([]byte, 0, 128),
			bodyBuf:         make([]byte, 0, 512),
			loc:             make([]byte, 0, 256),
			padding:         make([]byte, 4),
			executors:       make([]func(l *Log), 0, 8), // Reduced from 128 to 8 - typical logs have 3-5 executors (Time, Level, Location, PSM, LogID)
			psm:             make([]byte, 0, 16),
			kvlist:          make([]*writer.KeyValue, 0, 8), // Increased from 2 to 8 to reduce reallocation
			callDepthOffset: 0,
		}
	},
}

var (
	enableSecMark bool // whether KV to {{key=val}} or key=val
	currVersion   string
	errorPrint    = rate.NewLimiter(rate.Every(osTime.Second), 1)
)

func init() {
	enableSecMark = false // special idc: DetermineRegion() == REGION_TTP || DetermineRegion() == REGION_GCP
	currVersion = "-"     // current image version: env.ImageVersion()
}

// Log is a log instance of execute each log printing.
// A Log instance is always bound to a specific Logger,
// But a Logger can get some log from the log pool.
type Log struct {
	level       Level
	logger      *logger
	buf         []byte
	executors   []func(*Log)
	ctx         context.Context
	bodyBuf     []byte
	line        *Line
	time        osTime.Time
	includeZone bool
	psm         []byte

	callDepthOffset    int
	loc                []byte
	writingCount       int64
	padding            []byte
	strikes            int // 撞击
	stackInfo          StackInfo
	enableDynamicLevel bool

	kvlist []*writer.KeyValue
}

func newLog(level Level, logger *logger) *Log {
	l := logPool.Get().(*Log)
	l.level = level
	l.logger = logger
	l.padding = logger.padding
	l.includeZone = logger.includeZoneInfo
	l.callDepthOffset = 0
	return l
}

func (l *Log) appendStrings(items ...string) {
	for _, item := range items {
		l.buf = append(l.buf, item...)
	}
}

// With uses to pass a context to each printing,
// it is able to log log id and span id.
func (l *Log) With(ctx context.Context) *Log {
	if l == nil {
		return nil
	}
	l.ctx = ctx
	if !l.logger.lazyHandleCtx {
		l.handleCtx()
	}
	return l
}

// Str prints some strings in the log.
func (l *Log) Str(contents ...string) *Log {
	if l == nil {
		return nil
	}
	// avoid string literals escaping
	for _, s := range contents {
		l.bodyBuf = append(l.bodyBuf, s...)
	}
	l.bodyBuf = append(l.bodyBuf, l.padding...)
	return l
}

// Int prints some integers in the log.
func (l *Log) Int(is ...int) *Log {
	if l == nil {
		return nil
	}
	for _, i := range is {
		l.bodyBuf = strconv.AppendInt(l.bodyBuf, int64(i), 10)
		l.bodyBuf = append(l.bodyBuf, ',')
	}
	l.bodyBuf = append(l.bodyBuf[:len(l.bodyBuf)-1], l.padding...)
	return l
}

// Int64 prints some Int64 in the log.
func (l *Log) Int64(is ...int64) *Log {
	if l == nil {
		return nil
	}
	for _, i := range is {
		l.bodyBuf = strconv.AppendInt(l.bodyBuf, i, 10)
		l.bodyBuf = append(l.bodyBuf, ',')
	}
	l.bodyBuf = append(l.bodyBuf[:len(l.bodyBuf)-1], l.padding...)
	return l
}

// Float prints some floats in the log.
func (l *Log) Float(fs ...float64) *Log {
	if l == nil {
		return nil
	}
	for _, f := range fs {
		l.bodyBuf = strconv.AppendFloat(l.bodyBuf, f, 'f', -1, 64)
		l.bodyBuf = append(l.bodyBuf, ',')
	}
	l.bodyBuf = append(l.bodyBuf[:len(l.bodyBuf)-1], l.padding...)
	return l
}

// Bool prints a boolean in the log.
func (l *Log) Bool(b bool) *Log {
	if l == nil {
		return nil
	}
	switch b {
	case true:
		l.bodyBuf = append(l.bodyBuf, "true"...)
	default:
		l.bodyBuf = append(l.bodyBuf, "false"...)
	}
	l.bodyBuf = append(l.bodyBuf, l.padding...)
	return l
}

// Obj prints a object uses json.Marshal by default
// and prefer fmt.Stringer  if the structure implements it,
// remind that fmt.Stringer would cause extra malloc here.
func (l *Log) Obj(o interface{}, ops ...objOption) *Log {
	if l == nil {
		return nil
	}

	// The following code can avoid conf to be allocated to heap
	conf := logConf{}
	// check the length, otherwise there will be a memory allocation.
	if len(ops) > 0 {
		conf = genObjConf(ops)
	}
	if l.logger.convertObjToKV {
		conf.objStatus = convertObjToKV
	}

	switch conf.objStatus {
	case convertObjToKV:
		l.KV("object", o)
	default:
		switch v := o.(type) {
		case fmt.Stringer:
			value := reflect.ValueOf(o)
			if !value.IsValid() || value.Kind() == reflect.Ptr && value.IsNil() {
				return l.Str(fmt.Sprintf("%#v", o))
			}
			l.bodyBuf = append(l.bodyBuf, v.String()...)
		case error:
			value := reflect.ValueOf(o)
			if !value.IsValid() || value.Kind() == reflect.Ptr && value.IsNil() {
				return l.Str(fmt.Sprintf("%#v", o))
			}
			return l.Error(v)
		case float64:
			return l.Float(v)
		case int:
			return l.Int(v)
		case string:
			return l.Str(v)
		default:
			buf := (*jsonBuf)(unsafe.Pointer(&l.bodyBuf))
			e := json.NewEncoder(buf)
			e.SetEscapeHTML(false)
			err := e.Encode(v)
			if err != nil {
				return l.Str(fmt.Sprintf("%#v", o))
			}
			// encode would write '\n' in the end, remove it
			l.bodyBuf = buf.buf[:len(buf.buf)-1]
		}
		l.bodyBuf = append(l.bodyBuf, l.padding...)
	}
	return l
}

// Error prints an error in the log.
func (l *Log) Error(err error, ops ...errOption) *Log {
	if l == nil {
		return nil
	}

	// The following code can avoid conf to be allocated to heap
	conf := logConf{}
	// check the length, otherwise there will be a memory allocation.
	if len(ops) > 0 {
		conf = genErrConf(ops)
	}
	if l.logger.convertErrToKV {
		conf.errStatus = convertErrToKV
	}
	switch conf.errStatus {
	case convertErrToKV:
		l.KV("error", err)
	default:
		l.bodyBuf = append(l.bodyBuf, "<Error: "...)
		if value := reflect.ValueOf(err); !value.IsValid() || err == nil || value.Kind() == reflect.Ptr && value.IsNil() {
			l.bodyBuf = append(l.bodyBuf, "nil"...)
		} else {
			l.bodyBuf = append(l.bodyBuf, err.Error()...)
		}
		l.bodyBuf = append(l.bodyBuf, '>')
		l.bodyBuf = append(l.bodyBuf, l.padding...)
	}
	return l
}

// Line uses a cached Line instance to avoid reflection overhead,
// if there is no Line provided, Log would try to call runtime.Caller each printing.
func (l *Log) Line(line *Line) *Log {
	if l == nil {
		return nil
	}
	l.line = line
	return l
}

// Location directly sets the file location of the log.
// The users must make sure the file location has the right format. Otherwise, StreamLog may fail to parse the log.
// It must be "{Filename}.go:{LineNumber}"
func (l *Log) Location(pos string) *Log {
	if l == nil {
		return nil
	}
	l.loc = l.loc[:0]
	l.loc = append(l.loc, pos...)
	return l
}

// KV creates a KV instance and appends it into the kv list.
// When the log is emitted, the kv list will be handled depending on cases.
// If security mark is enabled, it converts the kv to a string (pattern: {{key=value}})
// and appends it to the message body.
func (l *Log) KV(key interface{}, value interface{}, ops ...kvOption) *Log {
	if l == nil {
		return nil
	}

	conf := logConf{}
	// check the length, otherwise there will be a memory allocation.
	if len(ops) > 0 {
		conf = genKvConf(ops)
	}
	switch conf.kvStatus {
	case kvInMsg:
		if enableSecMark {
			l.appendPaddingIfNecessary().Str("{{").trimPadding(true).Obj(key).trimPadding(true).
				Str("=").trimPadding(true).Obj(value).trimPadding(true).Str("}}")
		} else {
			l.appendPaddingIfNecessary().Obj(key).trimPadding(true).
				Str("=").trimPadding(true).Obj(value)
		}
	default:
		kv := writer.NewOmniKeyValue(key, value)
		l.kvlist = append(l.kvlist, kv)
	}
	return l
}

func (l *Log) appendPaddingIfNecessary() *Log {
	if l == nil {
		return nil
	}
	lenBuf, lenPadding := len(l.bodyBuf), len(l.padding)
	if lenBuf == 0 || lenBuf < lenPadding || l.bodyBuf[lenBuf-1] == ' ' || (lenPadding > 0 && bytes.Compare(l.bodyBuf[lenBuf-lenPadding:lenBuf], l.padding) == 0) {
		return l
	}

	if lenPadding != 0 {
		l.bodyBuf = append(l.bodyBuf, l.padding...)
		return l
	}
	l.bodyBuf = append(l.bodyBuf, spaceByte)
	return l
}

func (l *Log) trimPadding(trimSuffixWhitespace ...bool) *Log {
	trimPadding(&l.bodyBuf, l.padding, trimSuffixWhitespace...)
	return l
}

// StrKV creates a KV instance for two strings and appends it into the kv list.
// When the log is emitted, the kv list will be handled depending on cases.
// If security mark is enabled, it converts the kv to a string (pattern: {{key=value}})
// and appends it to the message body.
func (l *Log) StrKV(key, value string) *Log {
	if l == nil {
		return nil
	}
	kv := writer.NewStrKeyValue(key, value, true)
	l.kvlist = append(l.kvlist, kv)
	return l
}

// EmplaceKV converts a kv pair into a string (key=value) and append it to the message.
// It is equivalent to use KV(key, value, AppendKVInMsg())
func (l *Log) EmplaceKV(key interface{}, value interface{}) *Log {
	if l == nil {
		return nil
	}
	return l.KV(key, value, AppendKVInMsg())
}

// KVs handles a KV list.
func (l *Log) KVs(kvlist ...interface{}) *Log {
	if l == nil {
		return nil
	}

	if len(kvlist) == 0 || (len(kvlist)&1 == 1) { // ignore odd kvlist
		return l
	}

	for i := 0; i+1 < len(kvlist); i += 2 {
		l = l.KV(kvlist[i], kvlist[i+1])
	}
	return l
}

// StrKVs handles a string list.
func (l *Log) StrKVs(kvlist ...string) *Log {
	if l == nil {
		return nil
	}

	if len(kvlist) == 0 || (len(kvlist)&1 == 1) { // ignore odd kvlist
		return l
	}

	for i := 0; i+1 < len(kvlist); i += 2 {
		l = l.StrKV(kvlist[i], kvlist[i+1])
	}
	return l
}

// EmplaceKVs converts a KV list into strings and append them to the message body.
func (l *Log) EmplaceKVs(kvlist ...interface{}) *Log {
	if l == nil {
		return nil
	}

	if len(kvlist) == 0 || (len(kvlist)&1 == 1) { // ignore odd kvlist
		return l
	}

	for i := 0; i+1 < len(kvlist); i += 2 {
		l = l.EmplaceKV(kvlist[i], kvlist[i+1])
	}
	return l
}

// Stack will add stack info when composing the log instance.
// If printAllGoroutines is true, it prints the stacks of all goroutines.
// Otherwise, it just prints the current goroutine's stack.
func (l *Log) Stack(printAllGoroutines bool) *Log {
	if l == nil {
		return nil
	}
	if printAllGoroutines {
		l.stackInfo = AllGoroutines
	} else {
		l.stackInfo = CurrGoroutine
	}
	return l
}

func (l *Log) CallDepth(d int) *Log {
	if l == nil {
		return nil
	}
	l.callDepthOffset = d
	return l
}

func (l *Log) setPadding(p []byte) *Log {
	if l == nil {
		return nil
	}
	l.padding = l.padding[:0]
	l.padding = append(l.padding, p...)
	return l
}

// Limit controls the rate of a log based on its file location.
// limit is the max frequency in a second. The rate limit is not accurate.
// You may find the actual number of logs slightly exceeds limit * time.
func (l *Log) Limit(limit int) *Log {
	if l == nil {
		return nil
	}
	if limit <= 0 {
		recycle(l)
		return nil
	}

	location := l.fetchLoc()
	if l.logger.rateLimiters.Allow(location, limit) {
		return l
	}
	recycle(l)
	return nil
}

// Emit emits log print.
func (l *Log) Emit() {
	if l == nil {
		return
	}

	if l.logger.lazyHandleCtx {
		l.handleCtx()
	}

	//if l.logger.addEnv {
	//	l.KVs(vendor_tags.TagEnv, env.Env(), vendor_tags.TagEnvType, getHostEnv())
	//}

	// Lazy execution here
	for _, e := range l.executors {
		e(l)
	}

	l.trimPadding(true)
	reader := (*logReader)(unsafe.Pointer(l))
	for _, middleware := range l.logger.middlewares { // We only have a metricMiddleware at this point
		readerLog := middleware(reader)
		if readerLog == nil {
			return
		}
	}

	if enableSecMark {
		for _, kv := range l.kvlist {
			k, v := kv.ToKV()
			l.appendPaddingIfNecessary().Str("{{", k, "=", v, "}}")
		}
		l.trimPadding(true)
		l.kvlist = l.kvlist[:0]
		versionKV, _ := writer.NewKeyValue("current_version", currVersion)
		l.kvlist = append(l.kvlist, versionKV)
	}

	switch l.stackInfo {
	case CurrGoroutine:
		buffer := bytes.NewBuffer(make([]byte, 0, 1024))
		pcs := make([]uintptr, _TracebackMaxFrames)
		_ = runtime.Callers(1, pcs)
		frames := runtime.CallersFrames(pcs)
		for {
			f, more := frames.Next()
			_, err := fmt.Fprintf(buffer, "#\t0x%x\t%s\t%s:%d\n", f.PC, f.Function, f.File, f.Line)
			if err != nil || !more {
				break
			}
		}
		stackKV, _ := writer.NewKeyValue("stack", buffer.String(), true)
		l.kvlist = append(l.kvlist, stackKV)
	case AllGoroutines:
		buffer := bytes.NewBuffer(make([]byte, 0, 4096))
		pprof.Lookup("goroutine").WriteTo(buffer, 1)
		stackKV, _ := writer.NewKeyValue("stack", buffer.String(), true)
		l.kvlist = append(l.kvlist, stackKV)
	default:
	}

	switch l.logger.kvPosition {
	case AfterMsg:
		l.appendStrings(*(*string)(unsafe.Pointer(&l.bodyBuf)))
		if len(l.kvlist) > 0 {
			l.appendStrings(" ")
			for _, kv := range l.kvlist {
				l.buf = kv.EncodeAsStr(l.buf)
				l.buf = append(l.buf, spaceByte)
			}
		}
	default:
		if len(l.kvlist) > 0 {
			for _, kv := range l.kvlist {
				l.buf = kv.EncodeAsStr(l.buf)
				l.buf = append(l.buf, spaceByte)
			}
		}
		l.appendStrings(*(*string)(unsafe.Pointer(&l.bodyBuf)))
	}

	if len(l.buf) == 0 {
		// This should not happen since users should not store the log instance.
		return
	}

	if l.buf[len(l.buf)-1] == spaceByte {
		l.buf = l.buf[:len(l.buf)-1]
	}

	currLevel, currLogger := l.level, l.logger
	atomic.AddInt64(&l.writingCount, int64(len(l.logger.writers)))
	for i, _ := range l.logger.writers {
		if !l.enableDynamicLevel && l.logger.writers[i].getLevel() > l.level {
			reader.Recycle()
			continue
		}
		err := l.logger.writers[i].Write(reader)
		if err != nil {
			if errorPrint.Allow() {
				_, _ = fmt.Fprintf(os.Stderr, "log writes error: %s\n", err)
			}
		}
	}

	if currLogger.exitWhenFatal && currLevel == FatalLevel {
		currLogger.Flush()
		os.Exit(1)
	}
}

// EmitEveryN emits the log based on the count of calls.
// It logs the 1st call, (N+1)st call, (2N+1)st call, etc.
func (l *Log) EmitEveryN(n int) {
	if l == nil {
		return
	}

	if n <= 0 {
		recycle(l)
		return
	}

	location := l.fetchLoc()
	if !l.logger.countLimiters.Allow(location, n) {
		recycle(l)
		return
	}

	l.Emit()
}

// handleCtx()
func (l *Log) handleCtx() *Log {
	if l == nil {
		return nil
	}

	if l.ctx == nil {
		return l
	}

	l = l.extractKVsFromCtx()
	if l.ctx != nil {
		if stackInfoValue := l.ctx.Value(stackInfoCtxKey); stackInfoValue != nil {
			if info, ok := stackInfoValue.(StackInfo); ok {
				l.stackInfo = info
			}
		}
	}
	return l
}

// extractKVsFromCtx extracts KVs from the context and process these KVs.
func (l *Log) extractKVsFromCtx() *Log {
	if l == nil {
		return nil
	}

	if l.ctx == nil {
		return l
	}

	if convertCtxKVListToStr {
		kvStr := GetAllKVsStr(l.ctx)
		if len(kvStr) == 0 {
			return l
		}
		return l.Str(kvStr)
	}

	kvList := GetAllKVs(l.ctx)
	return l.KVs(kvList...)
}

func (l *Log) fetchLoc() string {
	var fileLine string
	if len(l.loc) > 0 {
		return *(*string)(unsafe.Pointer(&l.loc))
	}
	if l.line != nil {
		f := l.line.load(l.logger.callDepth+l.callDepthOffset, l.logger.fullPath || enableSecMark)
		fileLine = *(*string)(unsafe.Pointer(&f))
	} else {
		pc, file, line, ok := runtime.Caller(l.logger.callDepth + l.callDepthOffset)
		if ok {
			if !(l.logger.fullPath || enableSecMark) {
				file = filepath.Base(file)
			}
			fileLine = file + ":" + strconv.Itoa(line)

			if l.logger.funcNameInfo != noPrintFunc {
				fn := runtime.FuncForPC(pc)
				funcName := fn.Name()
				switch l.logger.funcNameInfo {
				case funcNameOnly:
					funcName = filepath.Ext(funcName)
					funcName = strings.TrimPrefix(funcName, ".")
				}
				l.StrKV(funcNameKey, funcName)
			}
		} else {
			fileLine = "?:?"
		}
	}
	l.loc = append(l.loc, fileLine...)
	return fileLine
}
