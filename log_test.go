package logs

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	osTime "time"
	"unsafe"

	"github.com/stretchr/testify/assert"

	w "github.com/erickxeno/logs/writer"
)

var line1 Line
var line3 Line
var ctx = context.Background()
var fiveStrings = []string{"a", "b", "c", "d", "e"}
var fiveNumbers = []int{0, 1, 2, 3, 4}
var kvlist = []interface{}{"count", 0, "id", "1", "addr", "home", "percentage", 0.1, Lesson{name: "English", id: 100, add: "school"}, 4}

type Lesson struct {
	name string
	id   int
	add  string
}

type test struct {
	A string
}

type testWriter struct {
	t     *testing.T
	check []string
	i     int
	lock  sync.Mutex
}

func newTestWriter(t *testing.T, check []string) *testWriter {
	return &testWriter{
		t:     t,
		check: check,
		i:     0,
		lock:  sync.Mutex{},
	}
}

func (w *testWriter) Close() error {
	return nil
}

func (w *testWriter) Write(log w.RecyclableLog) error {
	defer log.Recycle()
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.i < len(w.check) {
		bufContains := strings.Contains(string(log.GetBody()), w.check[w.i])
		kvContains := kvListContains(log.GetKVListStr(), w.check[w.i])
		if kvContains {
			fmt.Printf("\"%v\" is in the kvlist instead of the body\n", w.check[w.i])
		}
		levelContains := strings.Contains(strings.ToLower(log.GetLevel()), strings.ToLower(w.check[w.i]))
		locationContains := strings.Contains(strings.ToLower(string(log.GetLocation())), strings.ToLower(w.check[w.i]))
		psmContains := strings.Contains(strings.ToLower(log.GetPSM()), strings.ToLower(w.check[w.i]))
		assert.True(w.t, bufContains || kvContains || levelContains || locationContains || psmContains)
	}
	w.i++
	return nil
}

func (w *testWriter) Flush() error {
	return nil
}

func kvListContains(kv []string, item string) bool {
	for i := 0; i+1 < len(kv); i += 2 {
		key, val := kv[i], kv[i+1]
		if strings.Contains(key+"="+val, item) {
			return true
		}
	}
	return false
}

func TestLogPrint(t *testing.T) {
	resetSecMarkAndVersion()
	err := errors.New("test")
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	ctx = context.WithValue(ctx, "K_SPANID", uint64(123456))
	logger := NewCLogger(SetWriter(DebugLevel, w.NewAsyncWriter(&testWriter{
		t,
		[]string{
			"test and aaa 1,2,3 1,2,3 3.1,4.2,5.3 <Error: test> false {\"A\":\"t\"}",
			"Info",
			"Notice",
			"Warn",
			"Error",
			"Fatal",
		},
		0,
		sync.Mutex{},
	}, false), w.NewConsoleWriter(), &w.NoopWriter{}))
	logger.Debug().With(ctx).Line(&line1).Str("test", " and aaa").Int(1, 2, 3).Int64(1, 2, 3).Float(3.1, 4.2, 5.3).Error(err).Bool(false).Obj(&test{"t"}).Emit()

	logger.Info().With(ctx).Line(&line1).Str("test", " and aaa").Int(1, 2, 3).Int64(1, 2, 3).Float(3.1, 4.2, 5.3).Error(err).Bool(false).Obj(&test{"t"}).Emit()
	logger.Notice().With(ctx).Line(&line1).Str("test", " and aaa").Int(1, 2, 3).Int64(1, 2, 3).Float(3.1, 4.2, 5.3).Error(err).Bool(false).Obj(&test{"t"}).Emit()
	logger.Warn().With(ctx).Line(&line1).Str("test", " and aaa").Int(1, 2, 3).Int64(1, 2, 3).Float(3.1, 4.2, 5.3).Error(err).Bool(false).Obj(&test{"t"}).Emit()
	logger.Error().With(ctx).Line(&line1).Str("test", " and aaa").Int(1, 2, 3).Int64(1, 2, 3).Float(3.1, 4.2, 5.3).Error(err).Bool(false).Obj(&test{"t"}).Emit()
	logger.Fatal().With(ctx).Line(&line1).Str("test", " and aaa").Int(1, 2, 3).Int64(1, 2, 3).Float(3.1, 4.2, 5.3).Error(err).Bool(false).Obj(&test{"t"}).Emit()
	err = logger.Flush()
	assert.Nil(t, err)
	err = logger.Close()
	assert.Nil(t, err)
}

func TestNilLog(t *testing.T) {
	err := errors.New("test")
	var log *Log
	(*prefixedLog)(unsafe.Pointer(log)).Level().Time().Version().Location().Host().PSM("test").LogID().Cluster().Stage().SpanID().End().With(ctx).Line(&line1).Str("test", "and aaa").Int(1, 2, 3).Float(3.1, 4.2, 5.3).Error(err).Bool(false).Obj(&test{"t"}).Emit()
}

func TestFileWriter(t *testing.T) {
	resetSecMarkAndVersion()
	//p, err := ioutil.TempDir("", "test")
	//assert.Nil(t, err)
	var err error
	p := "test/"
	logger := NewCLogger(SetWriter(DebugLevel, w.NewFileWriter(filepath.Join(p, "test.log"), w.Hourly)))
	logger.Info().Str("test").Emit()
	err = logger.Flush()
	assert.Nil(t, err)
	file, err := os.Open(filepath.Join(p, "test.log"))
	assert.Nil(t, err)
	c, err := ioutil.ReadAll(file)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(c), "Info "))
	//assert.True(t, strings.Contains(string(c), "v1(0) log_test.go"))
	assert.True(t, strings.Contains(string(c), "test"))
	_ = os.Remove(filepath.Join(p, "test.log"))
	osTime.Sleep(5 * osTime.Second)
	logger.Flush()
	logger.Close()
}

func TestLoggerWithDifferentLevelWriter(t *testing.T) {
	resetSecMarkAndVersion()
	var err error
	p := "test/"
	filepath1 := filepath.Join(p, "test1.log")
	filepath2 := filepath.Join(p, "test2.log")
	fileWriter1 := w.NewFileWriter(filepath1, w.Hourly)
	fileWriter2 := w.NewFileWriter(filepath2, w.Hourly)
	consoleWriter := w.NewConsoleWriter()
	logger := NewCLogger(AppendWriter(DebugLevel, fileWriter1), AppendWriter(ErrorLevel, fileWriter2), AppendWriter(DebugLevel, consoleWriter))
	log := logger.Info()
	log.Str("Test writers with different level").Emit()
	err = logger.Flush()
	assert.Nil(t, err)
	assert.Equal(t, log.writingCount, int64(0))
	osTime.Sleep(2 * osTime.Second)

	file1, err := os.Open(filepath1)
	assert.Nil(t, err)
	content1, err := ioutil.ReadAll(file1)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(content1), "Info "))
	assert.True(t, strings.Contains(string(content1), "log_test.go"))
	assert.True(t, strings.Contains(string(content1), "Test writers with different level"))
	file2, err := os.Open(filepath2)
	assert.Nil(t, err)
	content2, err := ioutil.ReadAll(file2)
	assert.Empty(t, content2)

	_ = os.Remove(filepath1)
	_ = os.Remove(filepath2)
	osTime.Sleep(5 * osTime.Second)
	logger.Flush()
	logger.Close()
}

func TestAsyncWriterWithDifferentChanSize(t *testing.T) {
	resetSecMarkAndVersion()
	var err error
	p := "test/"
	logger := NewCLogger(SetWriter(DebugLevel, w.NewAsyncWriterWithChanLen(w.NewFileWriter(filepath.Join(p, "test3.log"), w.Hourly), 5120, true)))
	logger.Info().Str("test").Emit()
	err = logger.Flush()
	assert.Nil(t, err)
	file, err := os.Open(filepath.Join(p, "test3.log"))
	assert.Nil(t, err)
	c, err := ioutil.ReadAll(file)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(c), "Info "))
	assert.True(t, strings.Contains(string(c), "log_test.go"))
	assert.True(t, strings.Contains(string(c), "test"))
	_ = os.Remove(filepath.Join(p, "test3.log"))
	osTime.Sleep(5 * osTime.Second)
	logger.Flush()
	logger.Close()
}

type logWriter struct {
	t *testing.T
}

func (w *logWriter) Close() error {
	return nil
}

func (w *logWriter) Write(log w.RecyclableLog) error {
	defer log.Recycle()
	assert.True(w.t, strings.Contains(string(log.GetContent()), "test and aaa 1,2,3 3.1,4.2,5.3 <Error: test> false {\"A\":\"t\"}"))
	assert.NotNil(w.t, log.GetTime())
	assert.True(w.t, strings.Contains(log.GetLine(), "log_test.go"))
	assert.Equal(w.t, log.GetLevel(), "Debug")
	assert.NotNil(w.t, log.GetContext())
	return nil
}

func (w *logWriter) Flush() error {
	return nil
}

func TestToWriterInterface(t *testing.T) {
	err := errors.New("test")
	ctx := context.Background()
	logger := NewCLogger(SetWriter(DebugLevel, &logWriter{t}, w.NewConsoleWriter()))
	logger.Debug().With(ctx).Line(&line3).Str("test", " and aaa").Int(1, 2, 3).Float(3.1, 4.2, 5.3).Error(err).Bool(false).Obj(&test{"t"}).Emit()
}

func TestAgentWriter(t *testing.T) {
	err := errors.New("test")
	ctx := context.Background()
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	ctx = context.WithValue(ctx, "K_SPANID", uint64(123456))
	logger := NewCLogger(SetWriter(DebugLevel, w.NewAgentWriter()))
	logger.Debug().With(ctx).Line(&line3).Str("test", "and aaa").Int(1, 2, 3).Int64(1, 2, 3).Float(3.1, 4.2, 5.3).Error(err).Bool(false).Obj(&test{"t"}).Emit()
	assert.Nil(t, logger.Close())
}

func TestSetCallDepth(t *testing.T) {
	var l Line
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(),
		newTestWriter(t, []string{
			"testing.go:",
			"testing.go:",
			"log_test.go:259", // Update this line number when you update this file
			"testing.go:",
		})), SetCallDepth(3))
	logger.Debug().Str("test").Emit()
	logger.Debug().Line(&l).Str("test").Emit()
	logger.Debug().CallDepth(-1).Str("test").Emit()
	cLogger := NewCompatLoggerFrom(logger)
	cLogger.Info("test")
}

func TestSetFullPath(t *testing.T) {
	var l Line
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(),
		newTestWriter(t, []string{
			"log_test.go:",
			"log_test.go:",
		})), SetFullPath(true))
	logger.Info().Str("test").Emit()
	logger.Info().Line(&l).Str("test").Emit()
}

func TestSetZoneInfo(t *testing.T) {
	var l Line
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(),
		newTestWriter(t, []string{
			"log_test.go:",
			"log_test.go:",
		})), SetFullPath(true), SetZoneInfo(true))
	logger.Info().Str("test").Emit()
	logger.Info().Line(&l).Str("test").Emit()
}

func TestNilValue(t *testing.T) {
	var err error = nil
	var b *bar
	var bs fmt.Stringer
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter()))
	logger.Debug().Error(err).Obj(b).Obj(bs).Emit()

	logger.Debug().Error(baz{}).Emit()
}

type bar struct {
	string
}

func (b *bar) String() string {
	return b.string
}

type baz struct {
	string
}

func (b baz) Error() string {
	return b.string
}

func resetSecMarkAndVersion() {
	defaultSecMark := false
	if enableSecMark != defaultSecMark {
		SetSecMark(defaultSecMark)
	}

	defaultVersion := "-"
	if strings.Compare(currVersion, defaultVersion) != 0 {
		SetCurrentVersion(defaultVersion)
	}
}

func TestLineWithOnlyFileName(t *testing.T) {
	l := LineWithOnlyFilename()
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(),
		newTestWriter(t, []string{
			"",
			"log_test.go",
		})), SetFullPath(true))
	logger.Info().Str("test").Emit()
	logger.Info().Line(l).Str("test").Emit()
}

func TestCustomLine(t *testing.T) {
	l := CustomLine([]byte("custom"))
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(),
		newTestWriter(t, []string{
			"",
			"custom",
		}),
	), SetFullPath(true))
	logger.Info().Str("test").Emit()
	logger.Info().Line(l).Str("test").Emit()
}

func TestKV_NilStringer(t *testing.T) {
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(),
		newTestWriter(t, []string{
			"a=(*logs.bar)(nil)",
		}),
	), SetFullPath(true))
	var a *bar
	logger.Info().KV("a", a).Emit()
}

func TestKV_NegativeInt(t *testing.T) {
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(),
		newTestWriter(t, []string{
			"a=-1",
			"b=-1",
			"c=-1",
		}),
	))
	logger.Info().KV("a", -1).Emit()
	logger.Info().KV("b", int32(-1)).Emit()
	logger.Info().KV("c", int64(-1)).Emit()
}

func TestKV_StrKV(t *testing.T) {
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(),
		newTestWriter(t, []string{
			"a=b",
			"b=c",
			"c=d",
		}),
	))
	logger.Info().StrKV("a", "b").Emit()
	logger.Info().StrKV("b", "c").Emit()
	logger.Info().StrKVs("a", "b", "c", "d").Emit()
}

func TestEmitEveryN(t *testing.T) {
	tw := newTestWriter(t, []string{
		"a=b",
		"b=c",
		"c=d",
		"a=b",
		"b=c",
		"c=d",
	})

	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(), tw))
	for i := 0; i < 10; i++ {
		logger.Info().KV("a", "b").EmitEveryN(5)
		logger.Info().StrKV("b", "c").EmitEveryN(5)
		logger.Info().StrKVs("a", "b", "c", "d").EmitEveryN(5)
	}

	for i := 0; i < 10; i++ {
		logger.Info().StrKVs("a", "b", "c", "d").EmitEveryN(1)
	}
	assert.Equal(t, 16, tw.i)
}

func TestLogOptions(t *testing.T) {
	tw := newTestWriter(t, []string{
		"object={\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"{\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
	})

	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(), tw))
	p := &person{"name", "id", 20}
	logger.Info().KV("a", "b", AppendKVInMsg()).Obj(p, ConvertObjToKV()).Emit()
	logger.Info().KV("a", "b").Obj(p).Emit()
}

func TestDynamicLogLevel(t *testing.T) {
	tw := newTestWriter(t, []string{
		"object={\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"{\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"object={\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"{\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"object={\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"{\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
	})

	logger := NewCLogger(SetWriter(WarnLevel, w.NewConsoleWriter(), tw))
	p := &person{"name", "id", 20}
	logger.Info().KV("a", "b", AppendKVInMsg()).Obj(p, ConvertObjToKV()).Emit()
	logger.Info().KV("a", "b").Obj(p).Emit()

	logger.Info(WithDynamicLoggerLevel(InfoLevel)).KV("a", "b", AppendKVInMsg()).Obj(p, ConvertObjToKV()).Emit()
	logger.Info(WithDynamicLoggerLevel(InfoLevel)).KV("a", "b").Obj(p).Emit()

	logger.Info().KV("a", "b", AppendKVInMsg()).Obj(p, ConvertObjToKV()).Emit()
	logger.Info().KV("a", "b").Obj(p).Emit()

	ctx := CtxAddKVs(context.Background(), "common_key0", "common_value")
	ctx = context.WithValue(ctx, DynamicLogLevelKey, InfoLevel)
	ctx = CtxAddKVs(ctx, "common_key1", "common_value")

	logger.Info(WithCtx(ctx)).KV("a", "b", AppendKVInMsg()).Obj(p, ConvertObjToKV()).Emit()
	logger.Info(WithCtx(ctx)).KV("a", "b").Obj(p).Emit()
}

func TestDisplayEnvInfo(t *testing.T) {
	//tw := newTestWriter(t, []string{
	//	"_env=prod",
	//	"_env=prod",
	//})
	//
	//logger := NewCLogger(SetWriter(InfoLevel, w.NewConsoleWriter(), tw), SetDisplayEnvInfo(true))
	//p := &person{"name", "id", 20}
	//logger.Info().KV("a", "b", AppendKVInMsg()).Obj(p, ConvertObjToKV()).Emit()
	//logger.Info().KV("a", "b").Obj(p).Emit()
}

func TestCompatLogger_DynamicLogLevel(t *testing.T) {
	tw := newTestWriter(t, []string{
		"object={\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"{\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"object={\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"{\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"object={\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
		"{\"Name\":\"name\",\"Id\":\"id\",\"Age\":20}",
	})

	logger := NewCompatLogger(SetWriter(WarnLevel, w.NewConsoleWriter(), tw), SetEnableDynamicLevel(true))
	p := &person{"name", "id", 20}
	logger.Debug("123")
	logger.Info("123")
	ctx := CtxAddKVs(context.Background(), "common_key0", "common_value")
	ctx = context.WithValue(ctx, DynamicLogLevelKey, InfoLevel)
	logger.CtxInfo(ctx, "%v=%v object=%v", "a", "b", p)
	logger.CtxInfo(ctx, "%v=%v %v", "a", "b", p)

	logger.CtxInfo(context.Background(), "%v=%v object=%v", "a", "b", p)
	logger.CtxInfo(context.Background(), "%v=%v %v", "a", "b", p)

	ctx = CtxAddKVs(ctx, "common_key1", "common_value")
	logger.CtxInfo(ctx, "%v=%v object=%v", "a", "b", p)
	logger.CtxInfo(ctx, "%v=%v %v", "a", "b", p)
}

func TestInvalidLimitAPI(t *testing.T) {
	tw := newTestWriter(t, []string{})
	logger := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(), tw), SetEnableDynamicLevel(true))
	logger.Info().Limit(-1).Str("123").Emit()
}

func TestCompatLogger_Pointer(t *testing.T) {
	tw := newTestWriter(t, []string{"123:0x"})
	logger := NewCompatLogger(SetWriter(WarnLevel, w.NewConsoleWriter(), tw), SetEnableDynamicLevel(true))
	p := &person{"name", "id", 20}
	logger.Warn("123:%p", p)
	logger.Flush()
}

func TestSetPSM_MetricsMiddleware(t *testing.T) {
	mockPsm := "ut.test.psm"
	metricRedirectWriter := &redirectWriter{}
	emittersV := newMetricMiddlewareEmitters(mockPsm)
	emitterMap.Store(mockPsm, emittersV)

	tw := newTestWriter(t, []string{mockPsm})
	logger := NewCLogger(SetWriter(WarnLevel, w.NewConsoleWriter(), tw), SetEnableDynamicLevel(true), SetPSM(mockPsm))

	logger.Error().Str("error").Emit()
	logger.Fatal().Str("fatal").Emit()

	logger.Flush()
	//emittersV.client.Flush()

	metricRedirectWriter.lock.Lock()
	defer metricRedirectWriter.lock.Unlock()
	//assert.Len(t, metricRedirectWriter.buf, 2)
	for _, v := range metricRedirectWriter.buf {
		assert.Contains(t, v, fmt.Sprintf("_psm=%s", mockPsm))
	}
}

type redirectWriter struct {
	lock sync.Mutex
	buf  []string
}

func (w *redirectWriter) Write(data []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.buf = append(w.buf, string(data))
	return len(data), nil
}

func (w *redirectWriter) Close() error {
	return nil
}

func TestCompatLogger_LineReuse(t *testing.T) {
	tw := newTestWriter(t, []string{"123:0x"})
	logger := NewCompatLogger(SetWriter(WarnLevel, w.NewConsoleWriter(), tw), SetEnableDynamicLevel(true))
	p := &person{"name", "id", 20}
	for i := 0; i < 10; i++ {
		logger.Warn("123:%p", p)
		// reuse in multi writer but not multi logger call
	}
	logger.Flush()
}

func TestInfo(t *testing.T) {
	Info("test")
}
