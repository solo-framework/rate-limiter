package config

import "ratelimiter/internal/validator"

type ConfigGroup struct {
	Name string

	// number of requests per second
	Rate float64

	// maximum number of requests allowed at once,
	// also sets the max number of tokens
	Burst int
}

type GroupList struct {
	list []ConfigGroup
}

func NewGroupList() *GroupList {
	return &GroupList{}
}

func (s *GroupList) List() []ConfigGroup {
	return s.list
}

func (s *GroupList) Count() int {
	return len(s.list)
}

func (s *GroupList) Add(name string, rateValue float64, burstValue int) error {
	if err := validator.ValidateGroup(name, rateValue, burstValue); err != nil {
		return err
	}

	s.list = append(s.list, ConfigGroup{
		Name:  name,
		Rate:  rateValue,
		Burst: burstValue,
	})

	return nil
}
