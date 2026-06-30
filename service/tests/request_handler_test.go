package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"ratelimiter/internal/errors"
	"ratelimiter/internal/logger"
	"ratelimiter/service"
	"ratelimiter/service/tests/mocks"
)

func TestNewRequestHandler(t *testing.T) {
	logger.InitLogger("dev")

	rh := service.NewRequestHandler(
		&service.Manager{},
		logger.GetLogger(),
	)

	require.NotNil(t, rh)
}

// func TestRequestHandler_Handle(t *testing.T) {
// 	type fields struct {
// 		limitManager *service.LimitManager
// 		logger       service.ILogger
// 	}
// 	type args struct {
// 		data []byte
// 	}
// 	tests := []struct {
// 		name   string
// 		fields fields
// 		args   args
// 		want   []byte
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			// s := &service.RequestHandler{
// 			// 	limitManager: tt.fields.limitManager,
// 			// 	logger:       tt.fields.logger,
// 			// }
// 			s := service.NewRequestHandler(tt.fields.limitManager, tt.fields.logger)
// 			if got := s.Handle(tt.args.data); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("RequestHandler.Handle() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

func TestRequestHandler_extractData(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		group   string
		user    string
		wantErr error
	}{
		// TODO: Add test cases.
		{
			name:    "ok",
			data:    []byte("default:userok"),
			group:   "default",
			user:    "userok",
			wantErr: nil,
		},
		{
			name:    "bad delimiter",
			data:    []byte("group_name=user_userid"),
			group:   "",
			user:    "",
			wantErr: errors.ErrInvalidData,
		},

		{
			name:    "bad input",
			data:    []byte("               :                    "),
			group:   "",
			user:    "",
			wantErr: errors.ErrInvalidData,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := service.NewRequestHandler(getDummyLimitManager(), newLogger())
			group, user, err := s.ExtractData(tt.data)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, errors.ErrInvalidData)
			}

			assert.Equal(t, tt.group, group)
			assert.Equal(t, tt.user, user)
		})
	}
}

func Test_Handle_With_Mock(t *testing.T) {
	t.Run("Handle with invalid data", func(t *testing.T) {
		s := service.NewRequestHandler(&errorLimitManager{}, newLogger())

		res := s.Handle([]byte("invalid data"))
		assert.Equal(t, []byte(service.RESPONSE_ERROR_INVALID_DATA), res)
	})

	ctrl := gomock.NewController(t)
	mock := mocks.NewMockIManager(ctrl)

	t.Run("Handle with unknown group", func(t *testing.T) {
		mock.EXPECT().Allow(gomock.Any(), gomock.Any()).Return(false, service.ErrGroupNotFound).Times(1)

		s1 := service.NewRequestHandler(mock, newLogger())
		res := s1.Handle([]byte("default:userok"))
		assert.Equal(t, []byte(service.RESPONSE_ERROR_GROUP_NOT_FOUND), res)
	})

	t.Run("Handle with random error", func(t *testing.T) {
		mock.EXPECT().Allow(gomock.Any(), gomock.Any()).Return(false, fmt.Errorf("random error")).Times(1)

		s1 := service.NewRequestHandler(mock, newLogger())
		res := s1.Handle([]byte("default:userok"))
		assert.Equal(t, []byte(service.RESPONSE_ERROR_INTERNAL), res)
	})

	t.Run("Handle returns OK", func(t *testing.T) {
		// check Allow() - OK

		fakeLim := fakeLimitManager{
			lim:         service.NewLimiter(1000, 1000),
			returnAllow: true,
		}

		rh := service.NewRequestHandler(&fakeLim, newLogger())
		result := rh.Handle([]byte("default:userok"))
		assert.Equal(t, []byte(service.RESPONSE_OK), result)
	})

	t.Run("Handle returns RESPONSE_LIMIT", func(t *testing.T) {
		// check Allow() - RESPONSE_LIMIT

		fakeLim := fakeLimitManager{
			lim:         service.NewLimiter(1000, 1000),
			returnAllow: false,
		}

		rh := service.NewRequestHandler(&fakeLim, newLogger())
		result := rh.Handle([]byte("default:userok"))
		assert.Equal(t, []byte(service.RESPONSE_LIMIT), result)
	})
}

var _ service.IManager = (*errorLimitManager)(nil)

type errorLimitManager struct{}

// Allow implements [service.IManager].
func (e *errorLimitManager) Allow(groupName, userid string) (bool, error) {
	panic("unimplemented in errorLimitManager")
}

// GetLimiter implements [service.IManager].
func (e *errorLimitManager) GetLimiter(groupName, userid string) (*service.Limiter, error) {
	return nil, service.ErrGroupNotFound
}

func getDummyLimitManager() *service.Manager {
	return &service.Manager{}
}

type fakeLimitManager struct {
	lim         *service.Limiter
	returnAllow bool
}

// Allow implements [service.IManager].
func (s *fakeLimitManager) Allow(groupName, userid string) (bool, error) {
	// panic(" fakeLimitManager unimplemented")
	return s.returnAllow, nil
}

func (s *fakeLimitManager) GetLimiter(groupName, userid string) (*service.Limiter, error) {
	return s.lim, nil
}
