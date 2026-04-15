package service

import "errors"

var ErrTooManyConnections = errors.New("too many connections")
var ErrClosed = errors.New("listener closed")

var ErrGroupNotFound = errors.New("group not found")
var ErrGroupExists = errors.New("group already exists")
var ErrNoGroups = errors.New("group list is empty")
var ErrLimiterNotFound = errors.New("limiter not found")

var ErrReadTimeout = errors.New("read from connection timeout")
var ErrWriteTimeout = errors.New("write to connection timeout")
var ErrPacketTooLarge = errors.New("packet too large")

var ErrShutdown = errors.New("all connections are closed by timeout")
