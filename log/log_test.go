package log

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/erickxeno/logs"
	"github.com/erickxeno/logs/writer"
	"github.com/stretchr/testify/assert"
)

func TestDefaultLogger(t *testing.T) {
	testV2LoggerSetup(t, true)
	SetDefaultLogger(logs.SetWriter(logs.InfoLevel, writer.NewConsoleWriter(writer.SetColorful(false))))
	ctx := context.TODO()
	CompatLogger.CtxInfo(ctx, "hello, %s", "world")
	CompatLogger.CtxDebug(ctx, "hello, %s", "world")
	V1.Info()
}

func TestV2V1CtxInfoSecMark(t *testing.T) {
	SetSecMark(true)
	assert.True(t, IsSecMarkEnabled())
	SetSecMark(false)
	assert.False(t, IsSecMarkEnabled())

	testV2LoggerSetup(t, true, []string{
		"count=42",
		"{{count=42}}",
		"name=bob",
		"{{count=100}}",
		"{{Name=Bob}}",

		"current_version=V1.0.1",
		"current_version=V1.0.1",
		"current_version=V1.0.1",
		"Name=Bob",
		"Name=Bob",
	}...,
	)
	ctx := context.Background()
	SetSecMark(true)
	CompatLogger.CtxInfo(ctx, "Hello world %s!", "count=42")
	CompatLogger.CtxInfo(ctx, "Hello world %s!", SecMark("count", 42))
	CompatLogger.CtxInfo(ctx, "Hello world %s!", SecMark("name", "bob"))
	V1.Info().With(ctx).Str("key-value:").KV("count", 100).Emit()
	V1.Info().With(ctx).Str("key-value:").KV("count", 100).KV("Name", "Bob").Emit()
	CompatLogger.Flush()

	ctx = logs.CtxAddKVs(ctx, "percent", 0.1, "id", "123456")
	SetCurrentVersion("V1.0.1")
	CompatLogger.CtxInfo(ctx, "Hello world %s!", "count=42")
	CompatLogger.CtxInfo(ctx, "Hello world %s!", SecMark("count", 42))
	CompatLogger.CtxInfo(ctx, "Hello world %s!", SecMark("name", "bob"))
	V1.Info().With(ctx).Str("key-value:").KV("count", 100).KV("Name", "Bob").Emit()
	V1.Info().With(ctx).Str("key-value:").KV("count", 100).KV("Name", "Bob").Emit()
	CompatLogger.Flush()
	SetSecMark(false)
}

func TestV2V2KVApi(t *testing.T) {
	testV2LoggerSetup(t, false, []string{
		"count=100",
		"Name=Bob",
	}...)
	ctx := context.Background()
	V1.Info().With(ctx).Str("key-value:").KV("count", 100).Emit()

	V1.Info().With(ctx).Str("key-value:").KV("count", 100).KV("Name", "Bob").Emit()

	testV2LoggerSetup(t, true, []string{
		"count=100",
		"{{Name=Bob}}",
	}...)

	V1.Info().With(ctx).Str("key-value:").KV("count", 100).KV("Name", "Bob").Emit()
}

func TestV2V2CtxApi(t *testing.T) {
	testV2LoggerSetup(t, true)
	ctx := context.Background()
	V1.Info().With(ctx).Str("Empty Ctx").Emit()

	ctx = logs.CtxAddKVs(ctx, "name", "Bob", "age", 20, "count", 100)
	V1.Info().With(ctx).Str("three kv pairs:").Emit()
}

type testWriter struct {
	t     *testing.T
	check []string
	i     int
}

func (w *testWriter) Close() error {
	return nil
}

func (w *testWriter) Write(log writer.RecyclableLog) error {
	defer log.Recycle()
	if w.i >= len(w.check) {
		return nil
	}
	bufContains := strings.Contains(string(log.GetContent()), w.check[w.i])
	kvContains := kvListContains(log.GetKVListStr(), w.check[w.i])

	if !bufContains && kvContains {
		fmt.Printf("\"%v\" is in kvlist instead of buf\n", w.check[w.i])
	}
	w.i += 1
	assert.True(w.t, bufContains || kvContains)

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

func testV2LoggerSetup(t *testing.T, enableKVList bool, expectedOutput ...string) {
	resetDefaultLogger()
	ops := make([]logs.Option, 0)
	if len(expectedOutput) != 0 {
		ops = append(ops, logs.AppendWriter(logs.DebugLevel, &testWriter{t, expectedOutput, 0}))
	}

	if len(ops) != 0 {
		SetDefaultLogger(ops...)
	}
}

func TestSetDefaultLoggerWithMiddleware(t *testing.T) {
	testV2LoggerSetup(t, true)
	SetDefaultLogger(logs.SetWriter(logs.InfoLevel, writer.NewConsoleWriter(writer.SetColorful(false))))
	ctx := context.TODO()
	CompatLogger.CtxWarn(ctx, "hello, %s", "world")
	CompatLogger.CtxError(ctx, "hello, %s", "world")

}

// Reset the default Logger
func resetDefaultLogger() {
	writers := make([]writer.LogWriter, 0)
	level := logs.DebugLevel
	ops := make([]logs.Option, 0)
	isInTCE := true //env.InTCE()
	psm := "psm"    //env.PSM()
	if isInTCE {
		level = logs.InfoLevel
		fileName := fmt.Sprintf("%s/%s/%s.log", userHomeDir(), logs.DefaultLogPath, psm)
		writers = append(writers, writer.NewAsyncWriter(writer.NewFileWriter(fileName, writer.Hourly), true))
		writers = append(writers, writer.NewAgentWriter())
	} else {
		writers = append(writers, writer.NewConsoleWriter(writer.SetColorful(true)))
	}
	ops = append(ops, logs.SetWriter(level, writers...))
	SetDefaultLogger(ops...)
}
