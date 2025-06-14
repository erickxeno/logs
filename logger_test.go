package logs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	osTime "time"

	"github.com/erickxeno/logs/writer"
	w "github.com/erickxeno/logs/writer"
	"github.com/stretchr/testify/assert"
)

func TestCLoggerKVAPIs(t *testing.T) {
	ctx := context.TODO()
	V2logger := NewCLogger(
		SetWriter(DebugLevel,
			&testWriter{
				t: t,
				check: []string{
					"hello",
					"count=100",
				},
			},
			w.NewConsoleWriter(w.SetColorful(true))),
	)

	V2logger.Debug().With(ctx).Str("hello").KV("count", 100).Emit()
	V2logger.Info().With(ctx).Str("hello").KV("count", 100).Emit()
	V2logger.Notice().With(ctx).Str("hello").KV("count", 100).Emit()
	V2logger.Warn().With(ctx).Str("hello").KV("count", 100).Emit()
	V2logger.Error().With(ctx).Str("hello").KV("count", 100).Emit()
	V2logger.Fatal().With(ctx).Str("hello").KV("count", 100).Emit()
}

func TestCLoggerSecMark(t *testing.T) {
	SetSecMark(true)
	assert.True(t, IsSecMarkEnabled())
	SetSecMark(false)
	assert.False(t, IsSecMarkEnabled())
	SetSecMark(true)
	logger1 := NewCLogger(
		SetWriter(DebugLevel, &testWriter{
			t: t,
			check: []string{
				"key={\"Bar\":\"Bar\",\"Baz\":666",
				"spStringifier_String",
			},
			i: 0,
		}, w.NewConsoleWriter()),
		SetFullPath(true))
	logger2 := NewCLogger(
		SetWriter(DebugLevel, &testWriter{
			t: t,
			check: []string{
				"key={\"Bar\":\"Bar\",\"Baz\":666",
				"spStringifier_String",
			},
			i: 0,
		},
			w.NewConsoleWriter(),
		),
		SetFullPath(true),
	)

	a := foobar{"Bar", 666}
	b := NewSPStringifier(a)
	c := person{"Bob", "id", 200}
	d := human{"Alice", 10}
	var e error
	f := errors.New("error")
	logger1.Info().KVs("key", a).Emit()
	logger1.Info().KVs("key", b).Emit()
	logger1.Info().KVs("key", c).Emit()
	logger1.Info().KVs("key", d).Emit()
	logger1.Info().KVs("key", e).Emit()
	logger1.Info().KVs("key", f).Emit()

	logger2.Info().KVs("key", a).Emit()
	logger2.Info().KVs("key", b).Emit()
	logger2.Info().KVs("key", c).Emit()
	logger2.Info().KVs("key", d).Emit()
	logger2.Info().KVs("key", e).Emit()
	logger2.Info().KVs("key", f).Emit()
	SetSecMark(false)
}

func TestLogger_RateLimitWriter(t *testing.T) {
	defer func() {
		os.RemoveAll("test/rate/")
	}()
	workerNum, duration := 2, 5
	dLogger := NewCLogger(
		SetWriter(DebugLevel,
			w.NewRateLimitWriter(w.NewAsyncWriter(w.NewFileWriter("test/rate/test_limit_2.log", w.Hourly), true), 200),
			w.NewRateLimitWriter(w.NewAsyncWriter(w.NewFileWriter("test/rate/test_limit_10.log", w.Hourly), true), 5),
			w.NewRateLimitWriter(w.NewAsyncWriter(w.NewFileWriter("test/rate/test_limit_invalid.log", w.Hourly), true), -1),
			w.NewAsyncWriter(w.NewRateLimitWriter(w.NewFileWriter("test/rate/test_limit_async.log", w.Hourly), 3), true),
			w.NewRateLimitWriter(&rateLimitWriterForTest{t: t, upperbound: int32(workerNum * (duration + 1) * (10 * 2))}, 10),
		),
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go rateWorker0(dLogger, "0", &wg)
	go rateWorker1(dLogger, "1", &wg)
	dLogger.Info().Str("main worker").Str("3").Emit()
	dLogger.Info().Str("main worker").Str("4").Emit()
	osTime.Sleep(osTime.Second * osTime.Duration(duration))
	wg.Done()
	dLogger.Flush()
}

func rateWorker0(dLogger *CLogger, id string, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 0; i < 1000; i++ {
		dLogger.Info().Str("job 0 with worker id: ").Str(id).Emit()
	}

}

func rateWorker1(dLogger *CLogger, id string, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)
	for i := 0; i < 1000; i++ {
		dLogger.Info().Str("job 1 with worker id: ").Str(id).Emit()
	}
}

func TestLogger_Limit(t *testing.T) {
	workerNum, duration, limit := 3, 5, 2

	dLogger := NewCLogger(
		SetWriter(DebugLevel,
			&rateLimitWriterForTest{t: t, upperbound: int32(workerNum * (duration + 1) * (limit * 2))},
		),
	)
	var wg sync.WaitGroup
	for i := 0; i < workerNum; i++ {
		wg.Add(1)
		switch i % 2 {
		case 0:
			go rateLimitWorker0(dLogger, strconv.Itoa(i), limit, &wg)
		case 1:
			go rateLimitWorker1(dLogger, strconv.Itoa(i), limit, &wg)
		default:
			fmt.Println("should not reach here")
		}
	}

	osTime.Sleep(osTime.Duration(duration) * osTime.Second)
	wg.Wait()
}

func rateLimitWorker0(dLogger *CLogger, id string, limit int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < 1000; i++ {
		dLogger.Info().Limit(limit).Str("job 0 with worker id: ").Str(id).Emit()
	}
}

func rateLimitWorker1(dLogger *CLogger, id string, limit int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < 1000; i++ {
		dLogger.Warn().Str("job 1 with worker id:").Str(id).Limit(limit).Emit()
	}
}

type rateLimitWriterForTest struct {
	t          *testing.T
	upperbound int32
	i          int32
}

func (w *rateLimitWriterForTest) Close() error {
	return nil
}

func (w *rateLimitWriterForTest) Write(log w.RecyclableLog) error {
	defer log.Recycle()
	atomic.AddInt32(&w.i, 1)
	val := atomic.LoadInt32(&w.i)
	assert.LessOrEqual(w.t, val, w.upperbound)
	return nil
}

func (w *rateLimitWriterForTest) Flush() error {
	return nil
}

func TestLogger_SetLevelForWriter(t *testing.T) {
	consoleWriter1 := w.NewConsoleWriter()
	consoleWriter2 := w.NewConsoleWriter()
	testWriter := &testWriter{
		t:     t,
		check: []string{"2", "4", "6"},
		i:     0,
	}
	writers := []w.LogWriter{consoleWriter1, consoleWriter2, testWriter}
	logger := NewCLogger(
		SetWriter(InfoLevel, writers...),
	)

	logger.Debug().Str("1").Emit()
	logger.Info().Str("2").Emit()
	logger.SetLevel(ErrorLevel)
	logger.Info().Str("3").Emit()
	logger.SetLevelForWriters(DebugLevel, consoleWriter1, testWriter)
	logger.Debug().Str("4").Emit()
	logger.SetLevelForWriters(ErrorLevel, consoleWriter2, testWriter)
	logger.Warn().Str("5").Emit()
	logger.SetLevelForWriters(InfoLevel, testWriter)
	logger.Info().Str("6").Emit()
	logger.SetLevelForWriters(ErrorLevel, consoleWriter1)
	logger.Info().Str("7").Emit()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Debug().Str("1").Emit()
			logger.Info().Str("2").Emit()
			logger.SetLevel(ErrorLevel)
			logger.Info().Str("3").Emit()
			logger.SetLevelForWriters(DebugLevel, consoleWriter1, testWriter)
			logger.Debug().Str("4").Emit()
			logger.SetLevelForWriters(ErrorLevel, consoleWriter2, testWriter)
			logger.Warn().Str("5").Emit()
			logger.SetLevelForWriters(InfoLevel, testWriter)
			logger.Info().Str("6").Emit()
		}()
	}
	wg.Wait()
}

func TestFatalLogExit(t *testing.T) {
	if os.Getenv("FATAL_LOG_EXIT_TEST") == "1" {
		writers := make([]w.LogWriter, 0)
		writers = append(writers, w.NewConsoleWriter())
		writers = append(writers, w.NewAsyncWriter(w.NewFileWriter("./test/test_fatal_0.log", w.Hourly), true))
		writers = append(writers, w.NewAsyncWriter(w.NewFileWriter("./test/test_fatal_1.log", w.Hourly), true))
		writers = append(writers, w.NewAsyncWriter(w.NewFileWriter("./test/test_fatal_3.log", w.Hourly), true))

		logger := NewCLogger(
			SetWriter(DebugLevel, writers...),
			SetFatalOSExit(true),
		)

		logger.Warn().Str("Warn logs should not exit").Emit()
		logger.Fatal().Str("Fatal logs should exit").Emit()
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatalLogExit")
	cmd.Env = append(os.Environ(), "FATAL_LOG_EXIT_TEST=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatal("not exit")
}

func TestEmplaceKVs(t *testing.T) {
	ctx := context.Background()
	ctx = CtxAddKVs(ctx, "key1", "value1", "key2", 2)

	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter()))
	V1.Info().With(ctx).Str("key-value:").EmplaceKV("count", 100).EmplaceKVs("key3", "value3", "key4", 4).Emit()
	V1.Info().With(ctx).Emit()
	V1.Info().KV("count", 100).Emit()
}

func TestPushNotice(t *testing.T) {
	ntc := newNoticeKVs()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			for j := 0; j < 10; j++ {
				x := fmt.Sprintf("%v_%v", id, j)
				ntc.PushNotice(x, x)
			}
			wg.Done()
		}(i)
	}

	kvMap := make(map[string]struct{})
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			x := fmt.Sprintf("%v_%v", i, j)
			kvMap[x] = struct{}{}
		}
	}

	wg.Wait()
	kvs := ntc.KVs()
	for i := 0; i < len(kvs); i += 2 {
		k := kvs[i]
		v := kvs[i+1]
		if k != v {
			t.Fatal("err")
		}

		str := k.(string)
		delete(kvMap, str)
	}

	if len(kvMap) != 0 {
		fmt.Println(len(kvMap))
		t.Fatal("err")
	}
}

func TestNewNoticeCtx(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, LogIDCtxKey, "1111")
	ctx = NewNoticeCtx(ctx)
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter()))
	CtxPushNotice(ctx, "key1", "value1")
	V1.CtxFlushNotice(ctx)
	logger := NewCompatLoggerFrom(V1)
	logger.CtxFlushNotice(ctx)
}

func TestAsyncModifyKV(t *testing.T) {
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter()))
	alice := person{"Alice", "Alice_id", 0}
	var mtx sync.Mutex
	log := V1.Info().KV("person", alice)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			mtx.Lock()
			defer mtx.Unlock()
			for i := 0; i < 10; i++ {
				alice.Age += 1
			}
			wg.Done()
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		mtx.Lock()
		defer mtx.Unlock()
		log.Emit()
	}()
	wg.Wait()

}

func TestLog_Stack(t *testing.T) {
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false))), SetKVPosition(AfterMsg))
	V1.Info().Str("hello world").Stack(false).Emit()
	V1.Info().Str("hello world").Stack(true).Emit()
}

func TestLog_V2StackWithCtx(t *testing.T) {
	ctx := CtxAddKVs(context.Background(), "age", 20)
	ctx = CtxStackInfo(ctx, NoPrint)
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false))), SetKVPosition(AfterMsg))
	V1.Info().With(ctx).Str("hello world").Stack(true).Emit()
	V1.Info().With(ctx).Str("hello world").Stack(false).Emit()

	// not output stack info
	V1.Info().Str("hello world").Stack(true).With(ctx).Emit()
	V1.Info().Str("hello world").Stack(false).With(ctx).Emit()
}

func TestLog_V1StackWithCtx(t *testing.T) {
	ctx := CtxAddKVs(context.Background(), "age", 20)
	ctx = CtxStackInfo(ctx, CurrGoroutine)
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false))), SetKVPosition(AfterMsg))
	logger := NewCompatLoggerFrom(V1)

	logger.Error("hello %s v:%s %%112", "world", "v2")
	logger.CtxError(ctx, "hello %s", "world")
	logger.CtxErrorsf(ctx, "hello %s", "world")
	logger.CtxInfoKVs(ctx, "name", "bob")
}

func TestPrintStack(t *testing.T) {
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false))), SetKVPosition(AfterMsg))
	logger := NewCompatLoggerFrom(V1)
	logger.PrintStack(true)
	logger.PrintStack(false)
}

func TestCLogger_Location(t *testing.T) {
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false)), &testWriter{
		t:     t,
		check: []string{"location_test.go: 397", "location_test.go: 398"},
		i:     0,
	}), SetKVPosition(AfterMsg))
	l := &Line{}
	V1.Info().Line(l).Location("location_test.go: 397").Line(l).Emit()
	V1.Info().Line(l).Location("location_test.go: 398").Line(l).Emit()
}

func TestOmniKVApi(t *testing.T) {
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false)), &testWriter{
		t:     t,
		check: []string{"{\"Bar\":\"bar\",\"Baz\":10}=10", "{\"Bar\":\"bar\",\"Baz\":10}={\"Bar\":\"bar\",\"Baz\":20}"},
		i:     0,
	}), SetKVPosition(AfterMsg))
	err := fmt.Errorf("this is a error for test")
	V1.Info().KV(foobar{"bar", 10}, 10).Emit()
	V1.Info().KV(foobar{"bar", 10}, &foobar{"bar", 20}).Emit()
	V1.Info().KVs(10, &foobar{"bar", 20}, "err", err).Emit()
}

func TestConvertErrorToKV(t *testing.T) {
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false)),
		newTestWriter(t, []string{"{\"Bar\":\"bar\",\"Baz\":10}=10", "{\"Bar\":\"bar\",\"Baz\":10}={\"Bar\":\"bar\",\"Baz\":20}"}),
	), SetKVPosition(BeforeMsg), SetConvertErrorToKV(true))

	person := person{Name: "bob", Id: "1001", Age: 20}
	err := fmt.Errorf("this is a error for test")
	V1.Info().KV(foobar{"bar", 10}, 10).Emit()
	V1.Info().KV(foobar{"bar", 10}, &foobar{"bar", 20}).Emit()
	V1.Info().KVs(10, &foobar{"bar", 20}).Obj(person, ConvertObjToKV()).Emit()
	V1.Info().KVs(10, &foobar{"bar", 20}).Error(err, ConvertErrToKV()).Emit()
}

func TestConvertObjectToKV(t *testing.T) {
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false)),
		newTestWriter(t, []string{"{\"Bar\":\"bar\",\"Baz\":10}=10", "{\"Bar\":\"bar\",\"Baz\":10}={\"Bar\":\"bar\",\"Baz\":20}"}),
	), SetKVPosition(BeforeMsg), SetConvertObjectToKV(true))
	person := person{Name: "bob", Id: "1001", Age: 20}
	err := fmt.Errorf("this is a error for test")
	V1.Info().KV(foobar{"bar", 10}, 10).Emit()
	V1.Info().KV(foobar{"bar", 10}, &foobar{"bar", 20}).Emit()
	V1.Info().KVs(10, &foobar{"bar", 20}).Obj(person).Emit()
	V1.Info().KVs(10, &foobar{"bar", 20}).Error(err, ConvertErrToKV()).Emit()
}

func TestLoggerClose(t *testing.T) {
	writers := make([]writer.LogWriter, 0)
	level := DebugLevel
	ops := make([]Option, 0)

	level = InfoLevel
	writers = append(writers, writer.NewConsoleWriter())
	writers = append(writers, writer.NewAsyncWriter(&testWriter{
		t:     t,
		check: []string{"{\"Bar\":\"bar\",\"Baz\":10}=10", "{\"Bar\":\"bar\",\"Baz\":10}={\"Bar\":\"bar\",\"Baz\":20}"},
		i:     0,
	}, true))
	writers = append(writers, writer.NewAsyncWriter(&testWriter{
		t:     t,
		check: []string{"{\"Bar\":\"bar\",\"Baz\":10}=10", "{\"Bar\":\"bar\",\"Baz\":10}={\"Bar\":\"bar\",\"Baz\":20}"},
		i:     0,
	}, false))
	ops = append(ops, SetWriter(level, writers...))

	person := person{Name: "bob", Id: "1001", Age: 20}

	logger := NewCLogger(ops...)
	logger.Info().KV(foobar{"bar", 10}, 10).Emit()
	logger.Info().KV(foobar{"bar", 10}, &foobar{"bar", 20}).Emit()
	logger.Info().KVs(10, &foobar{"bar", 20}).Obj(person).Emit()
	logger.Close()

	go func() {
		for i := 0; i < 10; i++ {
			logger.Info().Str("should not be printed").Emit()
		}
	}()
	logger.Flush()
}

func TestPrintFuncName(t *testing.T) {
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(true)),
		newTestWriter(t, []string{"FuncForTest", "{\"Bar\":\"bar\",\"Baz\":10}=10", "{\"Bar\":\"bar\",\"Baz\":10}={\"Bar\":\"bar\",\"Baz\":20}"}),
	), SetKVPosition(BeforeMsg), SetDisplayFuncName(false))

	person := person{Name: "bob", Id: "1001", Age: 20}
	err := fmt.Errorf("this is a error for test")
	FuncForTest(V1)
	V1.Info().KV(foobar{"bar", 10}, 10).Emit()
	V1.Info().KV(foobar{"bar", 10}, &foobar{"bar", 20}).Emit()
	V1.Info().KVs(10, &foobar{"bar", 20}).Obj(person, ConvertObjToKV()).Emit()
	V1.Info().KVs(10, &foobar{"bar", 20}).Error(err, ConvertErrToKV()).Emit()
	V1.Close()

	V1 = NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false)),
		newTestWriter(t, []string{".FuncForTest", "{\"Bar\":\"bar\",\"Baz\":10}=10", "{\"Bar\":\"bar\",\"Baz\":10}={\"Bar\":\"bar\",\"Baz\":20}"}),
	), SetKVPosition(BeforeMsg), SetDisplayFuncName(true))

	FuncForTest(V1)
	V1.Info().KV(foobar{"bar", 10}, 10).Emit()
	V1.Info().KV(foobar{"bar", 10}, &foobar{"bar", 20}).Emit()
	V1.Info().KVs(10, &foobar{"bar", 20}).Obj(person, ConvertObjToKV()).Emit()
	V1.Info().KVs(10, &foobar{"bar", 20}).Error(err, ConvertErrToKV()).Emit()
}

func FuncForTest(logger *CLogger) {
	logger.Info().Str("test print func name").Emit()
}

func TestAbuse(t *testing.T) {
	tw := newTestWriter(t, []string{"{\"Bar\":\"bar\",\"Baz\":10}=10"})
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false)),
		tw,
	), SetKVPosition(BeforeMsg), SetDisplayFuncName(false))

	var log = V1.Info().KV(foobar{"bar", 10}, 10)
	log.Emit()
	log.Emit() // no output
	assert.Equal(t, 1, tw.i)
}

func TestFastRuntimeCaller(t *testing.T) {
	tw := newTestWriter(t, []string{
		"logger_test.go:533",
		"logger_test.go:535",
	})
	V1 := NewCLogger(SetWriter(DebugLevel, w.NewConsoleWriter(w.SetColorful(false)),
		tw,
	), SetKVPosition(BeforeMsg), SetDisplayFuncName(true))

	compatLogger := NewCompatLoggerFrom(V1)
	var log = V1.Info().KV(foobar{"bar", 10}, 10)
	log.Emit()
	assert.Equal(t, 1, tw.i)
	compatLogger.Info("hello %s", "world")
	compatLogger.Flush()
}

// Reset the default Logger
func resetDefaultLogger() {
	writers := make([]writer.LogWriter, 0)
	level := DebugLevel
	ops := make([]Option, 0)
	isInTCE := true //env.InTCE()
	psm := "psm"    //env.PSM()
	if isInTCE {
		level = InfoLevel
		fileName := fmt.Sprintf("%s/Documents/tiger/log/app/%s.log", userHomeDir(), psm)
		writers = append(writers, writer.NewAsyncWriter(writer.NewFileWriter(fileName, writer.Hourly), true))
		writers = append(writers, writer.NewAgentWriter())
		// for test
		writers = append(writers, writer.NewConsoleWriter(writer.SetColorful(true)))
	} else {
		writers = append(writers, writer.NewConsoleWriter(writer.SetColorful(true)))
	}
	ops = append(ops, SetWriter(level, writers...))
	SetDefaultLogger(ops...)
}

func TestDefaultLogger(t *testing.T) {
	SetDefaultLogger(
		SetEnableDynamicLevel(true),
		SetWriter(InfoLevel,
			w.NewConsoleWriter(w.SetColorful(true)), newTestWriter(t, []string{
				"logger_test.go:604",
				"data: 4",
				"data: 5",
				"data: 6",
				"data: 7",

				"data: 3",
				"data: 4",
				"data: 5",
				"data: 6",
				"data: 7",

				"logger_test.go:620",
				"data: 3",
				"data: 4",
				"data: 5",
				"data: 6",
				"data: 7",

				"logger_test.go:628",
				"data 2: 3",
				"data 2: 4",
				"data 2: 5",
				"data 2: 6",
				"data 2: 7",

				"age=2",
				"age=3",
				"age=4",
				"age=5",
				"age=6",
				"age=7",
			}),
		))
	EnableDynamicLogLevel()
	defer resetDefaultLogger()

	Trace("data: %d", 1)
	Debug("data: %d", 2)
	Info("data: %d", 3)
	Notice("data: %d", 4)
	Warn("data: %d", 5)
	Error("data: %d", 6)
	Fatal("data: %d", 7)

	Tracef("data: %d", 1)
	Debugf("data: %d", 2)
	Infof("data: %d", 3)
	Noticef("data: %d", 4)
	Warnf("data: %d", 5)
	Errorf("data: %d", 6)
	Fatalf("data: %d", 7)

	ctx := context.WithValue(context.Background(), DynamicLogLevelKey, DebugLevel)
	CtxTrace(ctx, "data: %d", 1)
	CtxDebug(ctx, "data: %d", 2)
	CtxInfo(ctx, "data: %d", 3)
	CtxNotice(ctx, "data: %d", 4)
	CtxWarn(ctx, "data: %d", 5)
	CtxError(ctx, "data: %d", 6)
	CtxFatal(ctx, "data: %d", 7)

	CtxTracesf(ctx, "data 2: %s", "1")
	CtxDebugsf(ctx, "data 2: %s", "2")
	CtxInfosf(ctx, "data 2: %s", "3")
	CtxNoticesf(ctx, "data 2: %s", "4")
	CtxWarnsf(ctx, "data 2: %s", "5")
	CtxErrorsf(ctx, "data 2: %s", "6")
	CtxFatalsf(ctx, "data 2: %s", "7")

	CtxTraceKVs(ctx, "age", 1)
	CtxDebugKVs(ctx, "age", 2)
	CtxInfoKVs(ctx, "age", 3)
	CtxNoticeKVs(ctx, "age", 4)
	CtxWarnKVs(ctx, "age", 5)
	CtxErrorKVs(ctx, "age", 6)
	CtxFatalKVs(ctx, "age", 7)
	PrintStack(false)
	Flush()
	Stop()
}

func TestDefaultCompatLogger_SetLevel(t *testing.T) {
	consoleWriter := w.NewConsoleWriter(w.SetColorful(true))
	testWriter := newTestWriter(t, []string{
		"logger_test.go:670",
		"logger_test.go:671",
		"logger_test.go:674",
		"logger_test.go:675",
	})
	consoleWriter2 := w.NewConsoleWriter(w.SetColorful(false))
	testWriter2 := newTestWriter(t, []string{
		"logger_test.go:671",
		"logger_test.go:674",
		"logger_test.go:675", "logger_test.go:678",
	})

	SetDefaultLogger(
		SetWriter(InfoLevel, consoleWriter, testWriter),
		AppendWriter(WarnLevel, consoleWriter2, testWriter2),
	)
	EnableDynamicLogLevel()
	defer resetDefaultLogger()

	ctx := CtxAddKVs(context.Background(), "key1", "value1")
	CtxInfo(ctx, "data: %d", 1)
	CtxWarn(ctx, "data: %d", 2)

	ctx2 := context.WithValue(ctx, DynamicLogLevelKey, DebugLevel)
	CtxInfo(ctx2, "data: %d", 3)
	CtxWarn(ctx2, "data: %d", 4)

	SetLevelForWriters(DebugLevel, consoleWriter2, testWriter2)
	CtxDebug(ctx, "data: %d", 5)
	SetLevel(ErrorLevel)
	CtxWarn(ctx, "data: %d", 6)
}

func TestInit(t *testing.T) {
	p1 := V1
	calldepth1 := V1.callDepth
	SetDefaultLogger(SetCallDepth(4))
	defer resetDefaultLogger()

	p2 := V1
	calldepth2 := V1.callDepth
	assert.Equal(t, p1, p2)
	assert.Equal(t, 2, calldepth1)
	assert.Equal(t, 4, calldepth2)
}

func TestGetWriter(t *testing.T) {
	consoleWriter := w.NewConsoleWriter(w.SetColorful(true))
	testWriter := newTestWriter(t, []string{})
	consoleWriter2 := w.NewConsoleWriter(w.SetColorful(false))
	testWriter2 := newTestWriter(t, []string{})

	SetDefaultLogger(
		SetWriter(InfoLevel, consoleWriter, testWriter),
		AppendWriter(WarnLevel, consoleWriter2, testWriter2),
	)
	EnableDynamicLogLevel()
	defer resetDefaultLogger()
	assert.Len(t, GetWriters(), 4)
}
