package writer

import (
	"fmt"
	"strconv"
	"testing"
	osTime "time"
)

func TestRateMap(t *testing.T) {
	rateBucketMap2 := NewRateLimiterMap()
	{
		for i := 0; i < 1; i++ {
			go worker(i, rateBucketMap2)
		}
		osTime.Sleep(4 * osTime.Second)
	}

	{
		for i := 0; i < 2; i++ {
			go worker(i, rateBucketMap2)
		}
		osTime.Sleep(5 * osTime.Second)
	}
}

func worker(id int, limiterMap RateLimiters) {
	for {
		osTime.Sleep(osTime.Millisecond * 100)
		fmt.Println("worker " + strconv.Itoa(id) + " check Allow.")
		if limiterMap.Allow(strconv.Itoa(id), 5) {
			fmt.Println("--------- worker " + strconv.Itoa(id) + " completed the job.")
		}
	}
}
