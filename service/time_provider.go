package service

import (
	"time"
)

type ITimeProvider interface {
	Now() int64
}

func NewTimeProvider() *TimeProvider {
	return &TimeProvider{}
}

type TimeProvider struct {
}

func (t *TimeProvider) Now() int64 {
	return time.Now().Unix()
}
