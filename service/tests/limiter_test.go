package tests

import (
	"fmt"
	"sync"
	"testing"

	"ratelimiter/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimiter_New(t *testing.T) {
	lim := service.NewLimiter(5, 10)

	require.NotNil(t, lim)
	assert.NotNil(t, lim.Get())
	assert.Equal(t, int64(0), lim.GetLastUse())
}

func TestLimiter_LastUse(t *testing.T) {
	lim := service.NewLimiter(1, 1)

	lim.SetLastUse(123)
	assert.Equal(t, int64(123), lim.GetLastUse())
}

func TestLimiter_Allow(t *testing.T) {
	lim := service.NewLimiter(1, 1)

	first := lim.Allow()
	second := lim.Allow()

	assert.True(t, first)
	assert.False(t, second)
}

func TestLimiter_LastUse_Concurrent(t *testing.T) {
	lim := service.NewLimiter(1, 1)

	const writers = 64
	const readers = 64

	start := make(chan struct{})
	errCh := make(chan error, readers)

	var wg sync.WaitGroup
	for i := range writers {
		val := int64(i)
		wg.Go(func() {
			<-start
			lim.SetLastUse(val)
		})
	}

	for range readers {
		wg.Go(func() {
			<-start
			got := lim.GetLastUse()
			if got < 0 || got >= writers {
				errCh <- fmt.Errorf("unexpected lastUse: %d", got)
			}
		})
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	lim.SetLastUse(999)
	assert.Equal(t, int64(999), lim.GetLastUse())
}
