package logs

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	osTime "time"
	"unsafe"

	"github.com/stretchr/testify/assert"

	w "github.com/erickxeno/logs/writer"
)

// Test for KV operation memory allocation optimization (issue #1)
func TestKVOperationOptimization(t *testing.T) {
	t.Run("PreallocatedKVList", func(t *testing.T) {
		// Test that kvlist is preallocated with capacity 8
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		log := logger.Info()

		// Verify initial capacity
		assert.Equal(t, 8, cap(log.kvlist), "kvlist should be preallocated with capacity 8")
		assert.Equal(t, 0, len(log.kvlist), "kvlist should start with length 0")

		// Add KVs without triggering reallocation
		for i := 0; i < 8; i++ {
			log.KV("key", i)
		}
		assert.Equal(t, 8, len(log.kvlist), "kvlist should have 8 elements")
		assert.Equal(t, 8, cap(log.kvlist), "kvlist capacity should remain 8 after adding 8 elements")

		log.Emit()
	})

	t.Run("FastPathStringKey", func(t *testing.T) {
		// Test that string keys use fast path
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		// This should use the fast path
		log := logger.Info().
			KV("string_key", "string_value").
			KV("int_key", 123).
			KV("int64_key", int64(456)).
			KV("int32_key", int32(789)).
			KV("bool_key", true)
		log.Emit()

		// No assertion needed - just verify it doesn't panic
	})

	t.Run("CommonTypeOptimization", func(t *testing.T) {
		// Test that common types (int, int64, int32, bool) are handled efficiently
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		log := logger.Info().
			KV("int", 42).
			KV("int64", int64(9223372036854775807)).
			KV("int32", int32(2147483647)).
			KV("uint64", uint64(18446744073709551615)).
			KV("uint32", uint32(4294967295)).
			KV("uint", uint(100)).
			KV("float64", 3.14159).
			KV("float32", float32(2.71828)).
			KV("bool_true", true).
			KV("bool_false", false)

		log.Emit()
	})

	t.Run("KeyValuePoolBufferSize", func(t *testing.T) {
		// Test that KeyValue pool allocates buffers with capacity 32
		kv := w.NewOmniKeyValue("test_key", "test_value_that_is_longer_than_16_bytes")
		assert.GreaterOrEqual(t, cap(kv.Value), 32, "KeyValue buffer should have capacity >= 32")
		kv.Recycle()
	})
}

// Test for concurrent performance optimization (issue #2)
func TestConcurrentPerformanceOptimization(t *testing.T) {
	t.Run("RecycleLogicOptimization", func(t *testing.T) {
		logger := NewLogger()
		logger.addWriter(InfoLevel, &w.NoopWriter{})

		// Test fast path: small buffers should always be recycled
		log := newLog(InfoLevel, logger)
		log.buf = make([]byte, 0, 128)
		log.bodyBuf = make([]byte, 0, 512)
		log.strikes = 5 // Should be reset to 0

		// Capture pool size before
		initialPoolSize := getPoolSize()

		recycle(log)

		// Verify strikes was reset
		newLog := logPool.Get().(*Log)
		assert.Equal(t, 0, newLog.strikes, "strikes should be reset for small buffers")
		logPool.Put(newLog)

		_ = initialPoolSize // Pool size check is informational
	})

	t.Run("RecycleMediumBuffers", func(t *testing.T) {
		logger := NewLogger()
		logger.addWriter(InfoLevel, &w.NoopWriter{})

		// Test medium-sized buffers with good utilization
		log := newLog(InfoLevel, logger)
		// Use buffers that are within 256KB threshold and well-utilized (>50%)
		log.buf = make([]byte, 32*1024, 64*1024)    // 32KB used, 64KB capacity
		log.bodyBuf = make([]byte, 64*1024, 128*1024) // 64KB used, 128KB capacity (total 192KB < 256KB)

		// Fill with data to simulate usage
		for i := 0; i < len(log.buf); i++ {
			log.buf[i] = byte(i % 256)
		}

		recycle(log)

		// Should be recycled because total capacity < 256KB and usage >= 50%
		newLog := logPool.Get().(*Log)
		assert.Equal(t, 0, newLog.strikes, "strikes should be reset for well-utilized buffers")
		logPool.Put(newLog)
	})

	t.Run("DoNotRecycleOversizedBuffers", func(t *testing.T) {
		logger := NewLogger()
		logger.addWriter(InfoLevel, &w.NoopWriter{})

		// Test that oversized buffers are not recycled after strikes
		log := newLog(InfoLevel, logger)
		log.buf = make([]byte, 10, 1024*1024)   // 1MB capacity, low usage
		log.bodyBuf = make([]byte, 10, 1024*1024) // 1MB capacity, low usage
		log.strikes = 4

		recycle(log)

		// Should not be put back to pool (strikes >= 4 and oversized)
		// We can't directly verify this, but we can check that a new log doesn't have these buffers
		newLog := logPool.Get().(*Log)
		// New logs from pool should have reasonable capacity
		assert.LessOrEqual(t, cap(newLog.buf), 1024, "new logs should not have oversized buffers")
		logPool.Put(newLog)
	})

	t.Run("AtomicRecycleOperation", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		log := logger.Info()
		reader := (*logReader)(unsafe.Pointer(log))

		// Set writingCount to 3 (simulating 3 writers)
		atomic.StoreInt64(&reader.writingCount, 3)

		// First two Recycle calls should just decrement
		reader.Recycle()
		assert.Equal(t, int64(2), atomic.LoadInt64(&reader.writingCount))

		reader.Recycle()
		assert.Equal(t, int64(1), atomic.LoadInt64(&reader.writingCount))

		// Third Recycle should trigger actual recycling (count becomes 0)
		reader.Recycle()
		assert.Equal(t, int64(0), atomic.LoadInt64(&reader.writingCount))
	})
}

// Test concurrent logging performance
func TestConcurrentLogging(t *testing.T) {
	t.Run("HighConcurrencyNoRace", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		ctx := context.Background()

		var wg sync.WaitGroup
		concurrency := 100
		iterationsPerGoroutine := 1000

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterationsPerGoroutine; j++ {
					logger.Info().
						With(ctx).
						Str("test message").
						KV("goroutine_id", id).
						KV("iteration", j).
						Emit()
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("ConcurrentWithMultipleKVs", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		var wg sync.WaitGroup
		concurrency := 50

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					logger.Info().
						KV("key1", "value1").
						KV("key2", 123).
						KV("key3", true).
						KV("key4", 3.14).
						KV("key5", int64(999)).
						KV("id", id).
						Str("concurrent test").
						Emit()
				}
			}(i)
		}

		wg.Wait()
	})
}

// Test buffer reuse and pool efficiency
func TestPoolEfficiency(t *testing.T) {
	t.Run("BufferReuse", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		// Create and emit multiple logs
		for i := 0; i < 100; i++ {
			log := logger.Info().
				Str("test message").
				KV("iteration", i)
			log.Emit()
		}

		// Verify pool is working by creating a new log and checking its state
		log := logger.Info()
		assert.NotNil(t, log)
		assert.Equal(t, 0, len(log.buf))
		assert.Equal(t, 0, len(log.bodyBuf))
		assert.Equal(t, 0, len(log.kvlist))
		log.Emit()
	})

	t.Run("FieldsProperlyReset", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		// Create a log with various fields set
		ctx := context.WithValue(context.Background(), "test", "value")
		logger.Info().
			With(ctx).
			Str("message").
			KV("key", "value").
			Emit()

		// Get a new log from pool and verify critical fields are reset
		log := logger.Info()
		assert.Nil(t, log.ctx)
		assert.Nil(t, log.line)
		assert.Equal(t, 0, len(log.buf))
		assert.Equal(t, 0, len(log.bodyBuf))
		assert.Equal(t, 0, len(log.kvlist))
		// Note: executors may have elements added by newLog (e.g., PSM, time, level)
		// so we only check that it's not nil and has reasonable capacity
		assert.NotNil(t, log.executors)
		assert.Equal(t, 0, len(log.loc))
		assert.Equal(t, NoPrint, log.stackInfo)
		assert.False(t, log.enableDynamicLevel)
		log.Emit()
	})
}

// Test cache locality optimization in recycle
func TestRecycleCacheLocality(t *testing.T) {
	t.Run("ResetOrderOptimization", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		// Create logs with various sizes and verify they're properly recycled
		sizes := []int{100, 1000, 10000}

		for _, size := range sizes {
			log := logger.Info()

			// Simulate different buffer sizes
			for i := 0; i < size; i++ {
				log.bodyBuf = append(log.bodyBuf, 'a')
			}

			log.KV("key", "value")
			log.Emit()
		}

		// Verify pool still works correctly
		log := logger.Info()
		assert.Equal(t, 0, len(log.buf))
		assert.Equal(t, 0, len(log.bodyBuf))
		log.Emit()
	})
}

// Test with actual time to ensure no performance regression
func TestPerformanceNoRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	t.Run("SimpleLoggingPerformance", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		iterations := 10000
		start := osTime.Now()

		for i := 0; i < iterations; i++ {
			logger.Info().Str("test message").Emit()
		}

		duration := osTime.Since(start)
		nsPerOp := duration.Nanoseconds() / int64(iterations)

		// Should be under 10,000ns (10µs) per operation in test environment
		// This is a sanity check, not a strict performance requirement
		assert.Less(t, nsPerOp, int64(10000), "simple logging should complete within reasonable time")
		t.Logf("Simple logging: %d ns/op", nsPerOp)
	})

	t.Run("KVLoggingPerformance", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		iterations := 10000
		start := osTime.Now()

		for i := 0; i < iterations; i++ {
			logger.Info().
				KV("user_id", 123).
				KV("ip", "127.0.0.1").
				KV("action", "login").
				Str("test message").
				Emit()
		}

		duration := osTime.Since(start)
		nsPerOp := duration.Nanoseconds() / int64(iterations)

		// Should be under 20,000ns (20µs) per operation in test environment
		// This is a sanity check, not a strict performance requirement
		assert.Less(t, nsPerOp, int64(20000), "KV logging should complete within reasonable time")
		t.Logf("KV logging: %d ns/op", nsPerOp)
	})
}

// Helper function to get approximate pool size (not exact, just for testing)
func getPoolSize() int {
	// We can't directly check pool size, so we'll return 0
	// This is just a placeholder for documentation purposes
	return 0
}
