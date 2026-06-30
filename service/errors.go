package service

import "errors"

var (
	ErrTooManyConnections = errors.New("too many connections")
	ErrClosed             = errors.New("listener closed")
)

var (
	ErrGroupNotFound   = errors.New("group not found")
	ErrGroupExists     = errors.New("group already exists")
	ErrNoGroups        = errors.New("group list is empty")
	ErrLimiterNotFound = errors.New("limiter not found")
)

var (
	ErrReadTimeout    = errors.New("read from connection timeout")
	ErrWriteTimeout   = errors.New("write to connection timeout")
	ErrPacketTooLarge = errors.New("packet too large")
)

var ErrShutdown = errors.New("all connections are closed by timeout")
