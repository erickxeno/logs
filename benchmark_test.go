package logs

import (
	"context"
	"errors"
	"io"
	"log"
	"math"
	"math/rand"
	"testing"

	w "github.com/erickxeno/logs/writer"
	"github.com/stretchr/testify/assert"
)

var line2 Line

func BenchmarkLogPrint(b *testing.B) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	writer := w.NewAsyncWriter(w.NewFileWriter("./test/test.log", w.Hourly), true)
	logger := NewCLogger(SetWriter(DebugLevel, writer))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Debug().
				With(ctx).
				Line(&line2).
				Str("Test logging, but use a somewhat realistic message length.").
				Int(fiveNumbers...).
				Str(fiveStrings...).
				Emit()
		}
	})
	b.StopTimer()
	err := logger.Flush()
	assert.Nil(b, err)
	err = logger.Close()
	assert.Nil(b, err)
}

func BenchmarkLogGeneration(b *testing.B) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	writer := &w.NoopWriter{}

	b.Run("v2_generate_log_with_line_api", func(b *testing.B) {
		logger := NewCLogger(SetWriter(DebugLevel, writer))
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().
					With(ctx).
					Line(&line2).
					Str("Test logging, but use a somewhat realistic message length.").
					Int(fiveNumbers...).
					Str(fiveStrings...).
					Emit()
			}
		})
	})

	b.Run("v2_generate_log_fast_caller", func(b *testing.B) {
		logger := NewCLogger(SetWriter(DebugLevel, writer))
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().
					With(ctx).
					Str("Test logging, but use a somewhat realistic message length.").
					Int(fiveNumbers...).
					Str(fiveStrings...).
					Emit()
			}
		})
	})

	b.Run("v2_generate_log", func(b *testing.B) {
		logger := NewCLogger(SetWriter(DebugLevel, writer))
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().
					With(ctx).
					Str("Test logging, but use a somewhat realistic message length.").
					Int(fiveNumbers...).
					Str(fiveStrings...).
					Emit()
			}
		})
	})

}

func BenchmarkFormatCompatLogger(b *testing.B) {
	logger := NewCompatLogger(SetWriter(DebugLevel, &w.NoopWriter{}))
	b.Run("Info", func(b *testing.B) {
		b.ReportAllocs()
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info("hello%s%d%f%%%#v", "world", 1, 0.1, f)
			}
		})
	})

	b.Run("Error", func(b *testing.B) {
		b.ReportAllocs()
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Error("hello%s%d%f%%%#v", "world", 1, 0.1, f)
			}
		})
	})

	b.Run("CtxInfoKvs", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.TODO()
		CtxAddKVs(ctx, "hello", "world")
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.CtxInfoKVs(ctx, "foo", f, "int", 1, "float", 0.4)
			}
		})
	})

	logger2 := NewCompatLogger(SetWriter(DebugLevel, &w.NoopWriter{}), SetEnableDynamicLevel(true))
	b.Run("Info_dynamic", func(b *testing.B) {
		b.ReportAllocs()
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger2.Info("hello%s%d%f%%%#v", "world", 1, 0.1, f)
			}
		})
	})
	b.Run("CtxInfoKvs_dynamic", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.TODO()
		CtxAddKVs(ctx, "hello", "world")
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger2.CtxInfoKVs(ctx, "foo", f, "int", 1, "float", 0.4)
			}
		})
	})

	logger3 := NewCompatLogger(SetWriter(DebugLevel, &w.NoopWriter{}), SetPSM("benchmark.set.psm"))
	b.Run("Info_set_psm", func(b *testing.B) {
		b.ReportAllocs()
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger3.Info("hello%s%d%f%%%#v", "world", 1, 0.1, f)
			}
		})
	})

	b.Run("Error_set_psm", func(b *testing.B) {
		b.ReportAllocs()
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger3.Info("hello%s%d%f%%%#v", "world", 1, 0.1, f)
			}
		})
	})

	b.Run("CtxInfoKvs_set_psm", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.TODO()
		CtxAddKVs(ctx, "hello", "world")
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger3.CtxInfoKVs(ctx, "foo", f, "int", 1, "float", 0.4)
			}
		})
	})
}

func BenchmarkCompatLoggerWithSecMark(b *testing.B) {
	SetSecMark(true)
	logger := NewCompatLogger(SetWriter(DebugLevel, &w.NoopWriter{}))
	b.Run("Info", func(b *testing.B) {
		b.ReportAllocs()
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info("hello%s%d%f%%%#v", "world", 1, 0.1, f)
			}
		})
	})
	b.Run("CtxInfoKvs", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.TODO()
		CtxAddKVs(ctx, "hello", "world")
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.CtxInfoKVs(ctx, "foo", f, "int", 1, "float", 0.4)
			}
		})
	})

	b.Run("CtxInfo", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.TODO()
		CtxAddKVs(ctx, "hello", "world")
		f := &foo{1, 0.1, "test"}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.CtxInfo(ctx, "key-value pairs: %s, %s, %s\n", SecMark("foo", f), SecMark("int", 1), SecMark("float", 0.4))
			}
		})
	})
	SetSecMark(false)
}

func BenchmarkNilLog(b *testing.B) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	writer := &w.NoopWriter{}

	logger := NewCLogger(SetWriter(InfoLevel, writer))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Debug().
				With(ctx).
				Line(&line2).
				Str("Test logging, but use a somewhat realistic message length.").
				Int(fiveNumbers...).
				Str(fiveStrings...).
				Emit()
		}
	})
	b.StopTimer()
}

func BenchmarkNilLogWithKVApis(b *testing.B) {
	SetSecMark(false)
	ctx := context.Background()
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	writer := &w.NoopWriter{}

	logger := NewCLogger(SetWriter(InfoLevel, writer))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Debug().
				With(ctx).
				Line(&line2).
				Str("Test logging, but use a somewhat realistic message length.").
				KVs(fiveStrings[0], fiveNumbers[0], fiveStrings[1], fiveNumbers[1],
					fiveStrings[2], fiveStrings[2], fiveStrings[3], fiveNumbers[3]).
				Emit()
		}
	})
	b.StopTimer()
}

func BenchmarkNilLogWithSecMark(b *testing.B) {
	SetSecMark(true)
	ctx := context.Background()
	ctx = context.WithValue(ctx, "K_LOGID", "1111")
	ctx = CtxAddKVs(ctx, "1", 1, "2", 2, "3", "3")
	writer := &w.NoopWriter{}

	logger := NewCLogger(SetWriter(InfoLevel, writer))

	b.Run("KVsApi", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Debug().
					With(ctx).
					KVs(kvlist).
					Emit()
			}
		})
	})

	b.Run("CtxApi", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.TODO()
		ctx = CtxAddKVs(ctx, "hello", "world", "Count", 100)
		var l Line
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Debug().
					Line(&l).
					With(ctx).
					Str("Test logging, but use a somewhat realistic message length.").
					Int(fiveNumbers...).
					Str(fiveStrings...).
					Emit()
			}
		})
	})

	SetSecMark(false)
}

func BenchmarkNoopWriter_WithRateLimit(b *testing.B) {
	writer := &w.NoopWriter{}
	logger := NewCLogger(SetWriter(DebugLevel, writer))

	b.Run("V2Limit", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		var l Line
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().
					Line(&l).
					Limit(math.MaxInt32).
					Str("worker id: ").
					Int(i).
					Emit()
			}
		})
	})

	logger2 := NewCLogger(
		SetWriter(DebugLevel, w.NewRateLimitWriter(writer, math.MaxInt32)),
	)

	b.Run("V2WithLimitWriter", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i := rand.Intn(2)
				switch i {
				case 0:
					logger2.Info().Str("worker id: ").Int(i).Emit()
				case 1:
					logger2.Info().Str("worker id: ").Int(i).Emit()
				}
			}
		})
	})

	logger3 := NewCLogger(
		SetWriter(DebugLevel, w.NewAsyncWriter(w.NewRateLimitWriter(writer, math.MaxInt32), true)),
	)

	b.Run("V2WithAsyncLimitWriter", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i := rand.Intn(2)
				switch i {
				case 0:
					logger3.Info().Str("worker id: ").Int(i).Emit()
				case 1:
					logger3.Info().Str("worker id: ").Int(i).Emit()
				}
			}
		})

	})
}

func BenchmarkXenoVsStdLog(b *testing.B) {
	// 准备测试数据
	testMsg := "This is a test message"
	testKV := map[string]interface{}{
		"user_id": 123,
		"ip":      "127.0.0.1",
		"action":  "login",
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, LogIDCtxKey, "logID")

	// 创建Xeno logger
	xenoLogger := NewCLogger(
		SetPSM("benchmark"),
	)

	// 创建标准库logger
	stdLogger := log.New(io.Discard, "", log.LstdFlags)

	b.Run("Xeno_Simple", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			xenoLogger.Info().Str(testMsg).Emit()
		}
	})

	b.Run("Std_Simple", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stdLogger.Print(testMsg)
		}
	})

	b.Run("Xeno_WithKV", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			log := xenoLogger.Info()
			for k, v := range testKV {
				log.KV(k, v)
			}
			log.Str(testMsg).Emit()
		}
	})

	b.Run("Std_WithKV", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stdLogger.Printf("%s user_id=%d ip=%s action=%s",
				testMsg,
				testKV["user_id"],
				testKV["ip"],
				testKV["action"],
			)
		}
	})

	b.Run("Xeno_WithContext", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			log := xenoLogger.Info().With(ctx)
			for k, v := range testKV {
				log.KV(k, v)
			}
			log.Str(testMsg).Emit()
		}
	})

	b.Run("Xeno_WithError", func(b *testing.B) {
		err := errors.New("test error")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			xenoLogger.Error().Error(err).Str(testMsg).Emit()
		}
	})

	b.Run("Std_WithError", func(b *testing.B) {
		err := errors.New("test error")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stdLogger.Printf("%s error=%v", testMsg, err)
		}
	})

	b.Run("Xeno_Concurrent", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				log := xenoLogger.Info()
				for k, v := range testKV {
					log.KV(k, v)
				}
				log.Str(testMsg).Emit()
			}
		})
	})

	b.Run("Std_Concurrent", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				stdLogger.Printf("%s user_id=%d ip=%s action=%s",
					testMsg,
					testKV["user_id"],
					testKV["ip"],
					testKV["action"],
				)
			}
		})
	})
}
