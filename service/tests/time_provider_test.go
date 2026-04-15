package tests

import (
	"ratelimiter/service"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeProvider_NowInRange(t *testing.T) {
	tp := service.NewTimeProvider()

	before := time.Now().Unix()
	got := tp.Now()
	after := time.Now().Unix()

	assert.GreaterOrEqual(t, got, before)
	assert.LessOrEqual(t, got, after)
}

func TestTimeProvider_NowMonotonicNonDecreasing(t *testing.T) {
	tp := service.NewTimeProvider()

	prev := tp.Now()
	for range 1000 {
		cur := tp.Now()
		assert.GreaterOrEqual(t, cur, prev)
		prev = cur
	}
}
