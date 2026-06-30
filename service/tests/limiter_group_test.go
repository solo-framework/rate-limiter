package tests

import (
	"strconv"
	"sync"
	"testing"

	"ratelimiter/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimiterGroup_AddGetRemoveCount(t *testing.T) {
	tp := &fixedTimeProvider{now: 100}
	group := service.NewLimiterGroup("g1", 10, 2, tp)

	assert.Equal(t, 0, group.Count())

	lim := group.Add("user1")
	require.NotNil(t, lim)
	assert.Equal(t, tp.now, lim.GetLastUse())
	assert.Equal(t, 1, group.Count())

	stored, ok := group.Get("user1")
	require.True(t, ok)
	require.NotNil(t, stored)

	group.Remove("user1")
	assert.Equal(t, 0, group.Count())

	_, ok = group.Get("user1")
	assert.False(t, ok)
}

func TestLimiterGroup_AllowUpdatesLastUse(t *testing.T) {
	tp := &fixedTimeProvider{now: 5}
	group := service.NewLimiterGroup("g1", 1, 2, tp)

	allowed := group.Allow("user1")
	assert.True(t, allowed)
	assert.Equal(t, 1, group.Count())

	tp.now = 42
	allowed = group.Allow("user1")
	assert.True(t, allowed)

	lim, ok := group.Get("user1")
	require.True(t, ok)
	assert.Equal(t, int64(42), lim.GetLastUse())
}

func TestLimiterGroup_ConcurrentAllowAndGet(t *testing.T) {
	tp := &fixedTimeProvider{now: 100}
	group := service.NewLimiterGroup("g1", 1000, 10, tp)

	const users = 200
	const iterations = 25

	errs := make(chan string, users)
	var wg sync.WaitGroup

	for i := range users {
		clientId := "user-" + strconv.Itoa(i)
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for j := range iterations {
				group.Allow(id)
				if j%5 == 0 {
					if _, ok := group.Get(id); !ok {
						errs <- id
						return
					}
				}
			}
		}(clientId)
	}

	wg.Wait()
	close(errs)
	for id := range errs {
		require.Fail(t, "missing limiter for user", id)
	}

	assert.Equal(t, users, group.Count())
	for i := range users {
		userID := "user-" + strconv.Itoa(i)
		lim, ok := group.Get(userID)
		require.True(t, ok)
		require.NotNil(t, lim)
	}
}

func TestLimiterGroup_ConcurrentRemoveAndCount(t *testing.T) {
	tp := &fixedTimeProvider{now: 100}
	group := service.NewLimiterGroup("g1", 1000, 10, tp)

	const users = 500
	for i := range users {
		group.Add("user-" + strconv.Itoa(i))
	}
	require.Equal(t, users, group.Count())

	start := make(chan struct{})
	done := make(chan struct{})
	errs := make(chan int, 1)

	var removeWG sync.WaitGroup
	for i := range users {
		userID := "user-" + strconv.Itoa(i)
		removeWG.Add(1)
		go func(id string) {
			defer removeWG.Done()
			<-start
			group.Remove(id)
		}(userID)
	}

	const countWorkers = 10
	var countWG sync.WaitGroup
	for range countWorkers {
		countWG.Go(func() {
			<-start
			for {
				select {
				case <-done:
					return
				default:
					c := group.Count()
					if c < 0 || c > users {
						select {
						case errs <- c:
						default:
						}
						return
					}
				}
			}
		})
	}

	close(start)
	removeWG.Wait()
	close(done)
	countWG.Wait()

	select {
	case c := <-errs:
		require.Failf(t, "unexpected count", "got %d", c)
	default:
	}

	assert.Equal(t, 0, group.Count())
}

func TestLimiterGroup_HighLoadConcurrentMethods(t *testing.T) {
	tp := &fixedTimeProvider{now: 100}
	group := service.NewLimiterGroup("g1", 1000, 10, tp)

	const users = 200
	const iterations = 2000
	const allowWorkers = 8
	const addWorkers = 4
	const removeWorkers = 4
	const countWorkers = 4

	ids := make([]string, users)
	for i := range users {
		ids[i] = "user-" + strconv.Itoa(i)
	}

	start := make(chan struct{})
	errCounts := make(chan int, 1)
	var wg sync.WaitGroup

	for i := range allowWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for j := range iterations {
				group.Allow(ids[(j+worker)%users])
			}
		}(i)
	}

	for i := range addWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for j := range iterations {
				group.Add(ids[(j+worker)%users])
			}
		}(i)
	}

	for i := range removeWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for j := range iterations {
				group.Remove(ids[(j+worker)%users])
			}
		}(i)
	}

	for range countWorkers {
		wg.Go(func() {
			<-start
			for range iterations {
				c := group.Count()
				if c < 0 || c > users {
					errCounts <- c
					return
				}
			}
		})
	}

	close(start)
	wg.Wait()

	select {
	case c := <-errCounts:
		require.Failf(t, "unexpected count", "got %d", c)
	default:
	}

	finalCount := group.Count()
	assert.GreaterOrEqual(t, finalCount, 0)
	assert.LessOrEqual(t, finalCount, users)
}

func TestLimiterGroup_MultithreadedAppSimulation(t *testing.T) {
	tp := &fixedTimeProvider{now: 123}
	group := service.NewLimiterGroup("g1", 500, 5, tp)

	const users = 120
	const hotUsers = 10
	const iterations = 1500
	const allowWorkers = 6
	const addWorkers = 3
	const removeWorkers = 3
	const getWorkers = 4
	const countWorkers = 2

	ids := make([]string, users)
	for i := range users {
		ids[i] = "user-" + strconv.Itoa(i)
	}

	for i := range hotUsers {
		group.Add(ids[i])
	}

	start := make(chan struct{})
	errs := make(chan string, 1)
	errCounts := make(chan int, 1)
	var wg sync.WaitGroup

	for i := range allowWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for j := range iterations {
				group.Allow(ids[(j+worker)%users])
			}
		}(i)
	}

	for i := range addWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for j := range iterations {
				idx := hotUsers + ((j + worker) % (users - hotUsers))
				group.Add(ids[idx])
			}
		}(i)
	}

	for i := range removeWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for j := range iterations {
				idx := hotUsers + ((j + worker) % (users - hotUsers))
				group.Remove(ids[idx])
			}
		}(i)
	}

	for i := range getWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for j := range iterations {
				id := ids[(j+worker)%users]
				if lim, ok := group.Get(id); ok && lim == nil {
					errs <- id
					return
				}
			}
		}(i)
	}

	for range countWorkers {
		wg.Go(func() {
			<-start
			for range iterations {
				c := group.Count()
				if c < 0 || c > users {
					errCounts <- c
					return
				}
			}
		})
	}

	close(start)
	wg.Wait()

	select {
	case id := <-errs:
		require.Fail(t, "nil limiter returned", id)
	default:
	}

	select {
	case c := <-errCounts:
		require.Failf(t, "unexpected count", "got %d", c)
	default:
	}

	for i := range hotUsers {
		lim, ok := group.Get(ids[i])
		require.True(t, ok)
		require.NotNil(t, lim)
	}
}
