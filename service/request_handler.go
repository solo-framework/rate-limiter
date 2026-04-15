package service

import (
	"errors"
	interrs "ratelimiter/internal/errors"
	"ratelimiter/internal/logger"
	"strings"
)

type IRequestHandler interface {

	// all business logic is handled here
	Handle(data []byte) []byte
}

// compile-time check that RequestHandler implements IRequestHandler.
var _ IRequestHandler = (*RequestHandler)(nil)

// implements IRequestHandler
type RequestHandler struct {
	manager IManager
	logger  logger.ILogger
}

func NewRequestHandler(limitManager IManager, logger logger.ILogger) *RequestHandler {
	return &RequestHandler{
		manager: limitManager,
		logger:  logger,
	}
}

// Handle extracts group name and user ID from the request,
// checks rate limit for the given group and user, and
// returns response based on the result of the check.
// Implements IRequestHandler interface.
// RESPONSE_OK is returned if the rate limit is not exceeded.
// RESPONSE_LIMIT is returned if the rate limit is exceeded.
// If the request is invalid, it returns RESPONSE_ERROR_INVALID_DATA.
// If the group is not found, it returns RESPONSE_ERROR_GROUP_NOT_FOUND response.
// If some internal error occurs, it returns RESPONSE_ERROR_INTERNAL response.
func (s *RequestHandler) Handle(data []byte) []byte {

	groupName, clientId, err := s.ExtractData(data)
	if err != nil {
		return []byte(RESPONSE_ERROR_INVALID_DATA)
	}

	allow, err := s.manager.Allow(groupName, clientId)

	if err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			return []byte(RESPONSE_ERROR_GROUP_NOT_FOUND)
		}

		s.logger.Error("RequestHandler.Handle error", "error", err)
		return []byte(RESPONSE_ERROR_INTERNAL)
	}

	response := RESPONSE_OK
	if !allow {
		response = RESPONSE_LIMIT
	}

	return []byte(response)
}

func (s *RequestHandler) ExtractData(data []byte) (string, string, error) {

	stringData := string(data)
	stringData = strings.TrimSpace(stringData)
	parts := strings.Split(stringData, ":")

	if len(parts) != 2 {
		return "", "", interrs.ErrInvalidData
	}

	group := parts[0]
	clientId := parts[1]
	if group == "" || clientId == "" {
		return "", "", interrs.ErrInvalidData
	}

	return group, clientId, nil
}
