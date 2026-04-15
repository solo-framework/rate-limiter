package service

import (
	"sync"
)

type LimiterGroup struct {
	name     string
	rateVal  float64
	burstVal int

	mu           sync.RWMutex
	store        map[string]*Limiter
	timeProvider ITimeProvider
}

func NewLimiterGroup(name string, rateVal float64, burstVal int, timeProv ITimeProvider) *LimiterGroup {
	return &LimiterGroup{
		store:        make(map[string]*Limiter),
		mu:           sync.RWMutex{},
		name:         name,
		rateVal:      rateVal,
		burstVal:     burstVal,
		timeProvider: timeProv,
	}
}

// Allow checks whether a request can be processed.
// If a limiter for clientId does not exist, it is created.
func (s *LimiterGroup) Allow(clientId string) bool {
	lim, ok := s.Get(clientId)

	if ok {
		lim.SetLastUse(s.timeProvider.Now())
		return lim.Allow()
	}

	lim = s.Add(clientId)
	return lim.Allow()
}

func (s *LimiterGroup) Get(clientId string) (*Limiter, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lim, ok := s.store[clientId]
	return lim, ok
}

// Add creates a new limiter and sets
// its last usage timestamp.
func (s *LimiterGroup) Add(clientId string) *Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	lim := NewLimiter(s.rateVal, s.burstVal)
	lim.SetLastUse(s.timeProvider.Now())

	s.store[clientId] = lim
	return lim
}

func (s *LimiterGroup) Remove(clientId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, clientId)
}

func (s *LimiterGroup) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.store)
}

// func (s *LimiterGroup) GetLimiters() map[string]*Limiter {
// 	return s.store
// }

// CleanUp removes limiters that have not been used for a long time.
// threshold is (NOW - TTL).
func (s *LimiterGroup) CleanUp(threshold int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for clientId, lim := range s.store {
		if lim.GetLastUse() <= threshold {
			delete(s.store, clientId)
		}
	}
}

func (s *LimiterGroup) Update(newRate float64, newBurst int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rateVal = newRate
	s.burstVal = newBurst

	for _, lim := range s.store {
		lim.Update(newRate, newBurst)
	}
}
