package tests

import (
	"fmt"
	cfg "ratelimiter/internal/config"
	"ratelimiter/service"
	"sync"
	"testing"
	"time"
)

func Benchmark_GetLimiter(b *testing.B) {

	if testing.Short() {
		b.Skipf("Benchmark_GetLimiter skipped in short mode")
	}

	config := getConfig()

	groups := cfg.NewGroupList()
	_ = groups.Add("default", 5_000_000, 1_000_000)
	_ = groups.Add("lala", 5_000_000, 1_000_000)

	manager, _ := service.NewManager(*groups, config.TTL, config.CleanupInterval, getTimeProvider())

	wg := sync.WaitGroup{}

	for i := 0; b.Loop(); i++ {

		userId := fmt.Sprintf("user_id_%d", i) // "user_id_" + string(i))
		grName := "default"
		if b.N%2 == 0 {
			grName = "lala"
		}

		wg.Add(1)
		go func() {

			wg.Done()
			time.Sleep(10 * time.Millisecond)
			_, _ = manager.GetLimiter(grName, userId)

		}()

	}

	wg.Wait()
}
