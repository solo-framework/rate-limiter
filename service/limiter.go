package service

import (
	"sync/atomic"

	"golang.org/x/time/rate"
)

// rate is the maximum requests per second (can be fractional)
// burst is the maximum number of requests allowed at once,
// and also the maximum token bucket size

type Limiter struct {
	limiter *rate.Limiter
	lastUse atomic.Int64
}

// NewLimiter returns a new Limiter with given rate and burst values.
// The rate is the maximum number of requests per second, and the burst is the maximum
// number of requests that can be executed at once.
func NewLimiter(rateVal float64, burstVal int) *Limiter {
	return &Limiter{
		limiter: rate.NewLimiter(rate.Limit(rateVal), burstVal),
		// lastUse: atomic.Int64{},
	}
}

func (s *Limiter) GetLastUse() int64 {
	return s.lastUse.Load()
}

// SetLastUse sets the Unix timestamp of the last limiter usage.
func (s *Limiter) SetLastUse(val int64) {
	s.lastUse.Store(val)
}

func (s *Limiter) Get() *rate.Limiter {
	return s.limiter
}

func (s *Limiter) Allow() bool {
	return s.limiter.Allow()
}

func (s *Limiter) Update(rateVal float64, burst int) {
	s.limiter.SetLimit(rate.Limit(rateVal))
	s.limiter.SetBurst(burst)
}
