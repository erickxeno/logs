package logs

import (
	"context"
	"fmt"
	"testing"

	w "github.com/erickxeno/logs/writer"
)

type foo struct {
	A int
	B float64
	C string
}

func (f *foo) String() string {
	return fmt.Sprintf("{\"A\":%d,\"B\":%d,\"C\":\"%s\"}", f.A, int(f.B), f.C)
}

func TestFastPrintf(t *testing.T) {
	logger := NewCLogger(SetPadding(""))
	logger.addWriter(DebugLevel, w.NewConsoleWriter())
	logger.Info().fastSPrintf("hello").Emit()
	logger.Info().fastSPrintf("hello %s int: %d float: %[3]*.[2]*[1]f% %% #v %t %f", "world", 1, 0.1, &foo{1, 0.1, "test"}, true, float32(0.1)).Emit()
	logger.Info().fastSPrintf("hello %s int: %d float: %[3]*.[2]*[1]f %% %#v %t %f", "world", 1, 0.1, &foo{1, 0.1, "test"}, true, float32(0.1)).Emit()
	logger.Info().fastSPrintf("%s", "").Emit()
	logger.Info().fastSPrintf("hello %v", &foo{1, 0.1, "test"}).Emit()
	logger.Info().fastSPrintf("hello %+v", &foo{1, 0.1, "test"}).Emit()
	logger.Info().fastSPrintf("hello %x", 17).Emit()
}

func TestCompat(t *testing.T) {
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	ctx = context.WithValue(ctx, "K_SPANID", "2222")

	l := NewCompatLogger(SetPSM("test.test.test"),
		SetWriter(DebugLevel, w.NewAgentWriter(), &testWriter{
			t: t,
			check: []string{
				"test.test.test",
				"test.test.test",
				"Debug",
				"Debug",
				"Warn",
				"Warn",
				"Notice",
				"Notice",
				"Fatal",
				"Fatal",
				"Error",
				"Error",
			},
		},
			w.NewConsoleWriter()),
	)
	l.Info("hello %v", "world")
	l.CtxInfo(ctx, "hello %v", "world")
	l.Debug("hello %v", "world")
	l.CtxDebug(ctx, "hello %v", "world")
	l.Warn("hello %v", "world")
	l.CtxWarn(ctx, "hello %v", "world")
	l.Notice("hello %v", "world")
	l.CtxNotice(ctx, "hello %v", "world")
	l.Fatal("hello %v", "world")
	l.CtxFatal(ctx, "hello %v", "world")
	l.Error("hello %s", fmt.Errorf("world"))
	l.CtxError(ctx, "hello %v", "world")
	l.Flush()
}

func TestCompatWithKVFlags(t *testing.T) {
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	ctx = context.WithValue(ctx, "K_SPANID", "2222")

	l := NewCompatLogger(SetPSM("test.test.test"),
		SetWriter(DebugLevel, w.NewAgentWriter(), &testWriter{
			t: t,
			check: []string{
				"test.test.test",
				"test.test.test",
				"Debug",
				"Debug",
				"Warn",
				"Warn",
				"Notice",
				"Notice",
				"Fatal",
				"Fatal",
				"Error",
				"Error",
			},
		},
			w.NewConsoleWriter()),
	)
	l.Info("hello %v", "world")
	l.CtxInfo(ctx, "hello %v", "world")
	l.Debug("hello %v", "world")
	l.CtxDebug(ctx, "hello %v", "world")
	l.Warn("hello %v", "world")
	l.CtxWarn(ctx, "hello %v", "world")
	l.Notice("hello %v", "world")
	l.CtxNotice(ctx, "hello %v", "world")
	l.Fatal("hello %v", "world")
	l.CtxFatal(ctx, "hello %v", "world")
	l.Error("hello %s", fmt.Errorf("world"))
	l.CtxError(ctx, "hello %v", "world")
	l.Flush()
}

func TestCompatKvs(t *testing.T) {
	ctx := context.TODO()
	l := NewCompatLogger(SetPSM("test.test.test"), SetWriter(DebugLevel, w.NewConsoleWriter(),
		newTestWriter(t, []string{
			"test={\"A\":1,\"B\":2,\"C\":\"3\"}",
			"hello=world",
			"test={\"A\":1,\"B\":2,\"C\":\"3\"}",
			"hello=world",
			"hello=world",
			"test={\"A\":1,\"B\":2,\"C\":\"3\"}",
		})),
	)
	ctx = CtxAddKVs(ctx, "test", &foo{1, 2, "3"}, "universe", 42)
	l.CtxFatalKVs(ctx, "hello", "world")
	l.CtxErrorKVs(ctx, "hello", "world")
	l.CtxWarnKVs(ctx, "hello", "world")
	l.CtxNoticeKVs(ctx, "hello", "world")
	l.CtxInfoKVs(ctx, "hello", "world")
	l.CtxDebugKVs(ctx, "hello", "world")
}

func TestCompatKvsWithKVFlags(t *testing.T) {
	ctx := context.TODO()
	l := NewCompatLogger(SetPSM("test.test.test"), SetWriter(DebugLevel, w.NewConsoleWriter(), &testWriter{
		t: t,
		check: []string{
			"test={\"A\":1,\"B\":2,\"C\":\"3\"}",
			"Error",
			"test={\"A\":1,\"B\":2,\"C\":\"3\"}",
			"Notice",
			"Info",
			"test={\"A\":1,\"B\":2,\"C\":\"3\"}",
		},
	}),
	)
	ctx = CtxAddKVs(ctx, "test", &foo{1, 2, "3"}, "universe", 42)
	l.CtxFatalKVs(ctx, "hello", "world")
	l.CtxErrorKVs(ctx, "hello", "world")
	l.CtxWarnKVs(ctx, "hello", "world")
	l.CtxNoticeKVs(ctx, "hello", "world")
	l.CtxInfoKVs(ctx, "hello", "world")
	l.CtxDebugKVs(ctx, "hello", "world")
}

func TestCompatKvsWithSecMark(t *testing.T) {
	ctx := context.TODO()
	l := NewCompatLogger(SetPSM("test.test.test"), SetWriter(DebugLevel, w.NewConsoleWriter(), &testWriter{
		t: t,
		check: []string{
			"test={\"A\":1,\"B\":2,\"C\":\"3\"}",
			"Error",
			"test={\"A\":1,\"B\":2,\"C\":\"3\"}",
			"Notice",
			"Info",
			"test={\"A\":1,\"B\":2,\"C\":\"3\"}",
		},
	}),
	)
	ctx = CtxAddKVs(ctx, "test", &foo{1, 2, "3"}, "universe", 42)
	l.CtxFatalKVs(ctx, "hello", "world")
	l.CtxErrorKVs(ctx, "hello", "world")
	l.CtxWarnKVs(ctx, "hello", "world")
	l.CtxNoticeKVs(ctx, "hello", "world")
	l.CtxInfoKVs(ctx, "hello", "world")
	l.CtxDebugKVs(ctx, "hello", "world")
}

func TestTracing(t *testing.T) {
	ctx := context.TODO()
	l := NewCompatLogger(SetPSM("test.test.test"), SetTracing())
	l.CtxTraceKVs(ctx, "test", "foo")
	l.Flush()
	l.Close()
}

func ExampleLazy() {
	l := NewCompatLogger(SetPSM("test.test.test"),
		SetWriter(InfoLevel, w.NewConsoleWriter()),
	)
	l.Debug("hello %v", Lazy(func() interface{} {
		panic("dead code")
	}))
}

func TestCompatiableAPI(t *testing.T) {
	V1.Info().Stack(false).Emit()
	Info("hello")
}
