package service

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"

	"ratelimiter/internal/config"
	"ratelimiter/internal/validator"
)

//go:generate mockgen -source=manager.go -destination=tests/mocks/manager_mock.go -package=mocks
type IManager interface {
	GetLimiter(groupName, clientId string) (*Limiter, error)
	Allow(groupName, clientId string) (bool, error)
}

var _ IManager = (*Manager)(nil)

type Manager struct {
	mu              sync.RWMutex
	groups          map[string]*LimiterGroup
	ttl             time.Duration
	cleanupInterval time.Duration
	stopCh          chan struct{}
	timeProv        ITimeProvider
}

func NewManager(groupsSettings config.GroupList, ttl, cleanupInterval time.Duration, timeProv ITimeProvider) (*Manager, error) {
	if len(groupsSettings.List()) == 0 {
		return nil, fmt.Errorf("NewManager error: %w", ErrNoGroups)
	}

	if ttl < 0 {
		return nil, fmt.Errorf("ttl mist be positive")
	}

	if cleanupInterval < 0 {
		return nil, fmt.Errorf("cleanupInterval mist be positive")
	}

	list := make(map[string]*LimiterGroup)

	for _, group := range groupsSettings.List() {
		list[group.Name] = NewLimiterGroup(group.Name, group.Rate, group.Burst, timeProv)
	}

	return &Manager{
		mu:              sync.RWMutex{},
		groups:          list,
		ttl:             ttl,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
		timeProv:        timeProv,
	}, nil
}

func (s *Manager) Allow(groupName, clientId string) (bool, error) {
	s.mu.RLock()
	group, ok := s.groups[groupName]
	if !ok {
		return false, fmt.Errorf("'Allow' access to group '%s': %w", groupName, ErrGroupNotFound)
	}
	s.mu.RUnlock()

	return group.Allow(clientId), nil
}

func (s *Manager) GetLimiter(groupName, clientId string) (*Limiter, error) {
	s.mu.RLock()
	group, ok := s.groups[groupName]
	if !ok {
		return nil, fmt.Errorf("'GetLimiter' access to group '%s': %w", groupName, ErrGroupNotFound)
	}
	s.mu.RUnlock()

	lim, ok := group.Get(clientId)
	if !ok {
		return nil, ErrLimiterNotFound
	}
	return lim, nil
}

func (s *Manager) StartCleanup() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()
	wg := sync.WaitGroup{}

	wg.Go(func() {
		for {
			select {
			case <-ticker.C:
				s.CleanUp()
			case <-s.stopCh:
				return
			}
		}
	})

	wg.Wait()
}

func (s *Manager) StopCleanup() {
	s.stopCh <- struct{}{}
}

// GetStat returns a map of group name with limiter counts.
func (s *Manager) GetStat() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]int)
	for g := range s.groups {
		res[g] = s.groups[g].Count()
	}

	return res
}

func (s *Manager) GetInfo() any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type data struct {
		Name  string  `json:"name"`
		Count int     `json:"count"`
		Rate  float64 `json:"rate"`
		Burst int     `json:"burst"`
	}

	res := make([]data, 0)
	for _, g := range s.groups {
		d := data{Name: g.name, Count: g.Count(), Rate: g.rateVal, Burst: g.burstVal}
		res = append(res, d)
	}
	return res
}

// CleanUp removes limiters that have not been used for a long time.
func (s *Manager) CleanUp() {
	wg := sync.WaitGroup{}
	threshold := s.timeProv.Now() - int64(s.ttl.Seconds())

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, group := range s.groups {
		wg.Add(1)

		go func(gr *LimiterGroup) {
			wg.Done()
			gr.CleanUp(threshold)
		}(group)
	}

	wg.Wait()
}

func (s *Manager) DeleteGroup(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.groups, name)
}

func (s *Manager) AddGroup(name string, rateValue float64, burstValue int) error {
	if err := validator.ValidateGroup(name, rateValue, burstValue); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.groups[name]; ok {
		return ErrGroupExists
	}

	s.groups[name] = NewLimiterGroup(name, rateValue, burstValue, s.timeProv)
	return nil
}

func (s *Manager) UpdateGroup(groupName string, newRate float64, newBurst int) error {
	if err := validator.ValidateGroup(groupName, newRate, newBurst); err != nil {
		return err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	group, ok := s.groups[groupName]
	if !ok {
		return fmt.Errorf("UpdateGroup '%s' error: %w", groupName, ErrGroupNotFound)
	}

	group.Update(newRate, newBurst)
	return nil
}

func (s *Manager) SaveGoupsToFile(dir string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type tmp struct {
		Name  string
		Rate  float64
		Burst int
	}

	toSave := make([]tmp, 0)
	for _, g := range s.groups {
		toSave = append(toSave, tmp{
			Name:  g.name,
			Rate:  g.rateVal,
			Burst: g.burstVal,
		})
	}

	file := "/data.bin"
	dir = strings.TrimSuffix(dir, "/") + file

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(toSave); err != nil {
		return fmt.Errorf("encode groups error: %w", err)
	}

	if err := os.WriteFile(dir, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("save groups to file error: %w", err)
	}
	return nil
}

func (s *Manager) LoadGroupsFromFile(dir string) (int, error) {
	count := 0
	file := "/data.bin"
	dir = strings.TrimSuffix(dir, "/") + file

	data, err := os.ReadFile(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return count, nil
		}
		return count, fmt.Errorf("load groups from file error: %w", err)
	}

	type tmp struct {
		Name  string
		Rate  float64
		Burst int
	}

	var loaded []tmp
	dec := gob.NewDecoder(bytes.NewReader(data))

	if err := dec.Decode(&loaded); err != nil {
		return count, fmt.Errorf("decode groups error: %w", err)
	}

	for _, g := range loaded {
		if err := s.AddGroup(g.Name, g.Rate, g.Burst); err != nil {

			if errors.Is(err, ErrGroupExists) {
				err = s.UpdateGroup(g.Name, g.Rate, g.Burst)
				if err != nil {
					return count, fmt.Errorf("load groups from file error[update]: %w", err)
				}
				continue
			}
			return count, fmt.Errorf("load groups from file error[add]: %w", err)
		}
	}

	return len(loaded), nil
}
