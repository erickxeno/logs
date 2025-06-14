package writer

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
)

func TestDeepCopyStrSlice(t *testing.T) {
	keys := []string{"key1", "", "key2", "value"}
	res := deepCopyStrSlice(keys)
	assert.Equal(t, len(keys), len(res))
	assert.Equal(t, keys, res)
}

func TestCountLimit(t *testing.T) {
	var limiter RateLimiters = NewCountLimiterMap()
	assert.True(t, limiter.Allow("utils.go:108", 2))
	assert.False(t, limiter.Allow("utils.go:108", 2))
	assert.True(t, limiter.Allow("utils.go:108", 2))
	assert.False(t, limiter.Allow("utils.go:108", 2))
}

func TestCountLimitConcurrency(t *testing.T) {
	var limiter RateLimiters = NewCountLimiterSyncMap()

	assert.True(t, limiter.Allow("utils.go:108", 2))
	assert.False(t, limiter.Allow("utils.go:108", 2))
	assert.True(t, limiter.Allow("utils.go:108", 2))
	assert.False(t, limiter.Allow("utils.go:108", 2))

	{
		wantsLimit := 2
		wantsGoroutineCount := 100
		wantsTryCount := 100

		var count int64
		var wg sync.WaitGroup
		wg.Add(wantsGoroutineCount)
		location := fmt.Sprintf("ultils.go:%d", 1)
		upperBound := int64(wantsTryCount*wantsGoroutineCount/wantsLimit + 1)

		for k := 0; k < wantsGoroutineCount; k++ {
			go func() {
				defer wg.Done()
				for j := 0; j < wantsTryCount; j++ {
					if limiter.Allow(location, wantsLimit) {
						atomic.AddInt64(&count, 1)
					}
				}
			}()
		}
		wg.Wait()
		fmt.Printf("N: %d, final outputed count: %v, upperbound: %d, location: %v, goroutine count: %v, each goroutine try: %v\n",
			wantsLimit, atomic.LoadInt64(&count), upperBound, location, wantsGoroutineCount, wantsTryCount)
		assert.LessOrEqual(t, atomic.LoadInt64(&count), upperBound)
		assert.GreaterOrEqual(t, atomic.LoadInt64(&count), upperBound-2)
		assert.Greater(t, atomic.LoadInt64(&count), int64(0))
	}

	fmt.Printf("-------------------\n")

	wants := []struct {
		limit          int
		goroutineCount int
		tryCount       int
	}{
		{1, 100, 100},
		{5, 100, 100},
		{6, 10, 100},
		{7, 10, 100},
		{8, 10, 100},
		{9, 100, 100},
		{10, 200, 100},
		{500, 400, 100},
		{1000, 200, 100},
		{10000, 300, 1000},
	}

	var wg2 sync.WaitGroup
	for i := range wants {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			var count int64
			var wg sync.WaitGroup
			wg.Add(wants[i].goroutineCount)
			location := fmt.Sprintf("ultils.go:%d", i)
			upperBound := int64(wants[i].tryCount*wants[i].goroutineCount/wants[i].limit + 1)

			for k := 0; k < wants[i].goroutineCount; k++ {
				go func() {
					defer wg.Done()
					for j := 0; j < wants[i].tryCount; j++ {
						if limiter.Allow(location, wants[i].limit) {
							atomic.AddInt64(&count, 1)
						}
					}
				}()
			}
			wg.Wait()
			fmt.Printf("N: %d, final outputed count: %v, upperbound: %d, location: %v, goroutine count: %v, each goroutine try: %v\n",
				wants[i].limit, atomic.LoadInt64(&count), upperBound, location, wants[i].goroutineCount, wants[i].tryCount)
			assert.LessOrEqual(t, atomic.LoadInt64(&count), upperBound)
			assert.GreaterOrEqual(t, atomic.LoadInt64(&count), upperBound-2)
			assert.Greater(t, atomic.LoadInt64(&count), int64(0))
		}(i)
	}
	wg2.Wait()
}

func TestMaxIntAtomicOp(t *testing.T) {
	var count uint64 = math.MaxUint64
	newValue := atomic.AddUint64(&count, 1)
	assert.Equal(t, uint64(0), newValue)
}

func BenchmarkCounterSyncMap(b *testing.B) {
	syncMap := NewCountLimiterSyncMap()

	limitMap := NewCountLimiterMap()

	b.Run("sync_map_benchmark", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		locations := genStrings(100)
		b.RunParallel(func(pb *testing.PB) {
			i := int64(0)
			for pb.Next() {
				newV := atomic.AddInt64(&i, 1)
				syncMap.Allow(locations[rand.Intn(100)], int(newV)%500)
			}
		})
	})

	b.Run("count_map_benchmark", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		locations := genStrings(100)
		b.RunParallel(func(pb *testing.PB) {
			i := int64(0)
			for pb.Next() {
				newV := atomic.AddInt64(&i, 1)
				limitMap.Allow(locations[rand.Intn(100)], int(newV)%500)
			}
		})
	})

}

func genStrings(count int) (res []string) {
	for i := 0; i < count; i++ {
		res = append(res, fmt.Sprintf("string_%d", i))
	}
	return res
}
