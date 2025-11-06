package logs

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/rs/zerolog"
	w "github.com/erickxeno/logs/writer"
)

// BenchmarkXenoVsZerolog compares performance between Xeno Logs and zerolog
func BenchmarkXenoVsZerolog(b *testing.B) {
	// 准备测试数据
	testMsg := "Test logging, but use a somewhat realistic message length"
	ctx := context.Background()
	ctx = context.WithValue(ctx, LogIDCtxKey, "test-log-id-12345")
	testErr := errors.New("test error message")

	// ========== Simple Logging ==========
	b.Run("Xeno_Simple", func(b *testing.B) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info().Str(testMsg).Emit()
		}
	})

	b.Run("Zerolog_Simple", func(b *testing.B) {
		logger := zerolog.New(io.Discard).With().Timestamp().Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info().Msg(testMsg)
		}
	})

	// ========== Logging with Key-Value Pairs ==========
	b.Run("Xeno_WithKV", func(b *testing.B) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info().
				Str(testMsg).
				KV("user_id", 12345).
				KV("ip", "192.168.1.100").
				KV("action", "login").
				KV("status", "success").
				Emit()
		}
	})

	b.Run("Zerolog_WithKV", func(b *testing.B) {
		logger := zerolog.New(io.Discard).With().Timestamp().Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info().
				Int("user_id", 12345).
				Str("ip", "192.168.1.100").
				Str("action", "login").
				Str("status", "success").
				Msg(testMsg)
		}
	})

	// ========== Logging with Context ==========
	b.Run("Xeno_WithContext", func(b *testing.B) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info().
				With(ctx).
				Str(testMsg).
				KV("user_id", 12345).
				KV("ip", "192.168.1.100").
				Emit()
		}
	})

	b.Run("Zerolog_WithContext", func(b *testing.B) {
		logger := zerolog.New(io.Discard).With().Timestamp().Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info().
				Ctx(ctx).
				Int("user_id", 12345).
				Str("ip", "192.168.1.100").
				Msg(testMsg)
		}
	})

	// ========== Error Logging ==========
	b.Run("Xeno_WithError", func(b *testing.B) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Error().
				Error(testErr).
				Str(testMsg).
				Emit()
		}
	})

	b.Run("Zerolog_WithError", func(b *testing.B) {
		logger := zerolog.New(io.Discard).With().Timestamp().Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Error().
				Err(testErr).
				Msg(testMsg)
		}
	})

	// ========== Concurrent Logging ==========
	b.Run("Xeno_Concurrent", func(b *testing.B) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().
					Str(testMsg).
					KV("user_id", 12345).
					KV("ip", "192.168.1.100").
					Emit()
			}
		})
	})

	b.Run("Zerolog_Concurrent", func(b *testing.B) {
		logger := zerolog.New(io.Discard).With().Timestamp().Logger()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().
					Int("user_id", 12345).
					Str("ip", "192.168.1.100").
					Msg(testMsg)
			}
		})
	})

	// ========== Disabled Log Level ==========
	b.Run("Xeno_Disabled", func(b *testing.B) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Debug().
				Str(testMsg).
				KV("user_id", 12345).
				Emit()
		}
	})

	b.Run("Zerolog_Disabled", func(b *testing.B) {
		logger := zerolog.New(io.Discard).With().Timestamp().Logger().Level(zerolog.InfoLevel)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Debug().
				Int("user_id", 12345).
				Msg(testMsg)
		}
	})

	// ========== Complex Logging Scenario ==========
	b.Run("Xeno_Complex", func(b *testing.B) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}), SetPSM("benchmark"))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info().
				With(ctx).
				Str(testMsg).
				KV("user_id", 12345).
				KV("ip", "192.168.1.100").
				KV("action", "login").
				KV("status", "success").
				KV("duration_ms", 156).
				KV("bytes_sent", 2048).
				Emit()
		}
	})

	b.Run("Zerolog_Complex", func(b *testing.B) {
		logger := zerolog.New(io.Discard).With().Timestamp().Str("service", "benchmark").Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info().
				Ctx(ctx).
				Int("user_id", 12345).
				Str("ip", "192.168.1.100").
				Str("action", "login").
				Str("status", "success").
				Int("duration_ms", 156).
				Int("bytes_sent", 2048).
				Msg(testMsg)
		}
	})
}
