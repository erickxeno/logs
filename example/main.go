package example

import (
	"context"
	"fmt"

	"github.com/erickxeno/logs"
	"github.com/erickxeno/logs/log"
)

var (
	// useful tool to reduce the overhead of file location getting
	l1 logs.Line
)

type foo struct {
	Bar string
	Baz int
}

func main() {
	ctx := context.TODO()
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	foo := &foo{"bar", 0}
	err := fmt.Errorf("an error")
	log.V1.Info().
		// a useful tool to avoid the overhead of runtime.Caller
		// it would boost the performance massively
		Line(&l1).
		// limit the emit rate of this log
		Limit(1).
		// pass context into a logger
		// log would record the log id and span id
		With(ctx).
		Str("hello world, ", "get:").
		Int(1).
		Float(4.2).
		Str("and bool:").
		Bool(true).
		// serialize object with encoding/json.Marshal
		Obj(foo).
		Str("with an error:").
		Error(err).
		KV("count", 100).
		KVs("Key1", "value1", "Key2", 0.1, "Key3", 3).
		// you have to emit to end up the method chain
		Emit()

	// print:
	// Info 2021-09-22 11:31:23,299 main.go:26 10.87.61.23 - 1111 default - 0 hello world, get: 1 4.2 and bool: true {"Bar":"bar","Baz":0} with an error: <Error: an error>  [[KVLIST]] count=100 Key1=value1 Key2=0.1 Key3=3
	log.CompatLogger.CtxInfo(ctx, "hello %s", "world")
	// Info 2020-12-17 21:20:20,696 main.go:70 10.86.111.73 - 1111 default - 0 hello world

	log.CompatLogger.Info("info without %v", ctx)
	// Info 2020-12-17 21:20:20,696 main.go:73 10.86.111.73 - - default - 0 info without {"Context":0}

	ctx = logs.CtxAddKVs(ctx, "bar", 1)
	log.CompatLogger.CtxInfoKVs(ctx, "foo", foo)
	// Info 2020-12-17 21:20:20,696 main.go:77 10.86.111.73 - - default - 0 bar=1 foo={"Bar":"bar","Baz":0}

	// close all client and graceful exit
	_ = log.Flush()

	log.CompatLogger.Info("hello %s", "world")
	logs.Info("hello %s", "world")
	log.CompatLogger.Info("hello %s", "world")
	logs.V1.Info().Str("hello world").Emit()
	logs.Flush()
}
