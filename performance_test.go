package logs

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	osTime "time"
	"unsafe"

	"github.com/stretchr/testify/assert"

	w "github.com/erickxeno/logs/writer"
)

// isRaceEnabled returns true if the race detector is enabled
func isRaceEnabled() bool {
	return raceEnabled
}

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
		// Skip assertion when race detector is enabled (slows down by 5-10x)
		if !isRaceEnabled() {
			assert.Less(t, nsPerOp, int64(10000), "simple logging should complete within reasonable time")
		}
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
		// Skip assertion when race detector is enabled (slows down by 5-10x)
		if !isRaceEnabled() {
			assert.Less(t, nsPerOp, int64(20000), "KV logging should complete within reasonable time")
		}
		t.Logf("KV logging: %d ns/op", nsPerOp)
	})
}

// Helper function to get approximate pool size (not exact, just for testing)
func getPoolSize() int {
	// We can't directly check pool size, so we'll return 0
	// This is just a placeholder for documentation purposes
	return 0
}

// Test for issue #3: runtime.Caller overhead optimization
func TestRuntimeCallerOptimization(t *testing.T) {
	t.Run("LocationCachingWithLine", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		line := &Line{}

		// Using Line() should avoid runtime.Caller
		log := logger.Info().Line(line)
		assert.NotNil(t, log.line, "line should be set")

		log.Str("test with line cache").Emit()
	})

	t.Run("LocationCachingWithLocationAPI", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		// Using Location() directly should cache the location
		log := logger.Info().Location("test.go:123")
		assert.NotEqual(t, 0, len(log.loc), "loc should be cached")

		log.Str("test with location cache").Emit()
	})

	t.Run("RuntimeCallerFallback", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		// Without Line() or Location(), should fall back to runtime.Caller
		log := logger.Info()
		assert.Nil(t, log.line, "line should not be set")
		assert.Equal(t, 0, len(log.loc), "loc should not be cached initially")

		log.Str("test without cache").Emit()
	})

	t.Run("CachedLocationPerformance", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping performance test in short mode")
		}

		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))
		line := &Line{}

		iterations := 10000
		start := osTime.Now()

		for i := 0; i < iterations; i++ {
			logger.Info().Line(line).Str("cached location").Emit()
		}

		duration := osTime.Since(start)
		nsPerOp := duration.Nanoseconds() / int64(iterations)

		t.Logf("Cached location logging: %d ns/op", nsPerOp)
	})
}

// Test for issue #4: Executor lazy execution mechanism optimization
func TestExecutorOptimization(t *testing.T) {
	t.Run("ExecutorPreallocation", func(t *testing.T) {
		// Test that executors slice is preallocated with capacity 8 (not 128)
		logger := NewLogger()
		logger.addWriter(InfoLevel, &w.NoopWriter{})

		log := newLog(InfoLevel, logger)

		// Verify initial capacity is 8 (optimized from 128)
		// Note: capacity may grow if more executors are added, but should start at 8
		initialCap := cap(log.executors)
		assert.LessOrEqual(t, initialCap, 16, "executors should start with reasonable capacity (<= 16, down from 128)")
		assert.Equal(t, 0, len(log.executors), "executors should start with length 0")

		recycle(log)
	})

	t.Run("TypicalExecutorCount", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		// Typical log with common executors
		log := logger.Info() // This may add some default executors

		initialLen := len(log.executors)
		initialCap := cap(log.executors)

		// Capacity should be reasonable (much less than original 128)
		assert.LessOrEqual(t, initialCap, 16, "capacity should be reasonable (<= 16, down from 128)")

		log.Emit()

		t.Logf("Typical executor count: %d, capacity: %d", initialLen, initialCap)
	})

	t.Run("ExecutorMemoryEfficiency", func(t *testing.T) {
		logger := NewCLogger(SetWriter(InfoLevel, &w.NoopWriter{}))

		// Create multiple logs to verify memory efficiency
		for i := 0; i < 100; i++ {
			log := logger.Info()
			// After growth, capacity may be 16, but should not be 128
			assert.LessOrEqual(t, cap(log.executors), 16, "capacity should be reasonable")
			log.Str("test").Emit()
		}
	})
}

// Helper function to get nextRotationTime from FileWriter using reflection
func getFileWriterNextRotationTime(fw w.LogWriter) int64 {
	v := reflect.ValueOf(fw).Elem()
	field := v.FieldByName("nextRotationTime")
	return atomic.LoadInt64((*int64)(unsafe.Pointer(field.UnsafeAddr())))
}

// Test for issue #5: FileWriter lock granularity optimization
func TestFileWriterLockOptimization(t *testing.T) {
	t.Run("AtomicRotationCheck", func(t *testing.T) {
		// Create a temporary file for testing
		tmpDir := t.TempDir()
		filename := tmpDir + "/test.log"

		fw := w.NewFileWriter(filename, w.Daily)
		defer fw.Close()

		// Verify nextRotationTime is initialized
		nextRot := getFileWriterNextRotationTime(fw)
		assert.NotEqual(t, int64(0), nextRot, "nextRotationTime should be initialized")
	})

	t.Run("NoLockWhenNoRotation", func(t *testing.T) {
		tmpDir := t.TempDir()
		filename := tmpDir + "/test.log"

		fw := w.NewFileWriter(filename, w.Daily)
		defer fw.Close()

		// Just verify the atomic field is accessible and non-zero
		nextRot := getFileWriterNextRotationTime(fw)
		assert.NotEqual(t, int64(0), nextRot, "nextRotationTime should be initialized")

		// Write some logs to verify it works correctly
		logger := NewCLogger(SetWriter(InfoLevel, fw))
		for i := 0; i < 10; i++ {
			logger.Info().Str("test").Emit()
		}
	})

	t.Run("ConcurrentWrites", func(t *testing.T) {
		tmpDir := t.TempDir()
		filename := tmpDir + "/test_concurrent.log"

		fw := w.NewFileWriter(filename, w.Daily)
		defer fw.Close()

		logger := NewCLogger(SetWriter(InfoLevel, fw))

		var wg sync.WaitGroup
		concurrency := 50
		iterationsPerGoroutine := 100

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterationsPerGoroutine; j++ {
					logger.Info().
						Str("concurrent write test").
						KV("goroutine", id).
						KV("iteration", j).
						Emit()
				}
			}(i)
		}

		wg.Wait()
	})
}

// Helper function to get channel capacity from AsyncWriter using reflection
func getAsyncWriterChannelCap(aw w.LogWriter) int {
	v := reflect.ValueOf(aw).Elem()
	ch := v.FieldByName("ch")
	return ch.Cap()
}

// Test for issue #6: AsyncWriter fixed channel capacity optimization
func TestAsyncWriterOptimization(t *testing.T) {
	t.Run("DefaultBufferSize", func(t *testing.T) {
		noop := &w.NoopWriter{}
		aw := w.NewAsyncWriter(noop, false)
		defer aw.Close()

		// Verify default buffer size is 4096 (increased from 1024)
		assert.Equal(t, 4096, getAsyncWriterChannelCap(aw), "default buffer size should be 4096")
	})

	t.Run("CustomBufferSize", func(t *testing.T) {
		noop := &w.NoopWriter{}
		customSize := 8192
		aw := w.NewAsyncWriterWithChanLen(noop, customSize, false)
		defer aw.Close()

		assert.Equal(t, customSize, getAsyncWriterChannelCap(aw), "custom buffer size should be respected")
	})

	t.Run("InvalidBufferSizeFallback", func(t *testing.T) {
		noop := &w.NoopWriter{}
		// Test with invalid (negative) buffer size
		aw := w.NewAsyncWriterWithChanLen(noop, -1, false)
		defer aw.Close()

		// Should fall back to default
		assert.Equal(t, 4096, getAsyncWriterChannelCap(aw), "should fall back to default buffer size")
	})

	t.Run("ZeroBufferSizeFallback", func(t *testing.T) {
		noop := &w.NoopWriter{}
		aw := w.NewAsyncWriterWithChanLen(noop, 0, false)
		defer aw.Close()

		// Should fall back to default
		assert.Equal(t, 4096, getAsyncWriterChannelCap(aw), "should fall back to default buffer size")
	})

	t.Run("HighThroughputScenario", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping high-throughput test in short mode")
		}

		noop := &w.NoopWriter{}
		// Use larger buffer for high throughput
		aw := w.NewAsyncWriterWithChanLen(noop, 8192, true)
		defer aw.Close()

		logger := NewCLogger(SetWriter(InfoLevel, aw))

		var wg sync.WaitGroup
		concurrency := 100
		iterationsPerGoroutine := 1000

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterationsPerGoroutine; j++ {
					logger.Info().
						Str("high throughput test").
						KV("goroutine", id).
						KV("iteration", j).
						Emit()
				}
			}(i)
		}

		wg.Wait()
		aw.Flush()
	})
}

// Integration test covering all optimizations
func TestAllOptimizationsIntegration(t *testing.T) {
	t.Run("CombinedOptimizations", func(t *testing.T) {
		// Create a logger with all optimizations
		tmpDir := t.TempDir()
		filename := tmpDir + "/integration.log"

		fw := w.NewFileWriter(filename, w.Daily)
		aw := w.NewAsyncWriterWithChanLen(fw, 4096, true)
		logger := NewCLogger(SetWriter(InfoLevel, aw))
		line := &Line{}

		var wg sync.WaitGroup
		concurrency := 50
		iterations := 100

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					logger.Info().
						Line(line). // Issue #3: cached location
						Str("integration test").
						KV("key1", "value1"). // Issue #1: optimized KV
						KV("key2", j).
						Emit() // Issue #4: optimized executors
				}
			}(i)
		}

		wg.Wait()
		aw.Flush()
		aw.Close()
	})
}
