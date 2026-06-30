package config

import (
	"testing"

	"ratelimiter/internal/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GroupList_List(t *testing.T) {
	list := NewGroupList()
	_ = list.Add("test1", 5, 10)
	_ = list.Add("test2", 5, 10)
	_ = list.Add("test3", 5, 10)

	if len(list.List()) != 3 {
		t.Errorf("GroupList.List: expected 3 groups, got %d", len(list.List()))
	}
}

func Test_GroupList_Add(t *testing.T) {
	type fields struct {
		name  string
		rate  float64
		burst int
	}

	type test struct {
		fields fields
		errStr string
		name   string
	}

	tests := []test{
		{
			fields: fields{
				name:  "test1",
				rate:  5,
				burst: 10,
			},
			errStr: "",
			name:   "OK",
		},
		{
			fields: fields{
				name:  "",
				rate:  5,
				burst: 10,
			},
			errStr: "group name must not be empty",
			name:   "Empty group name",
		},
		{
			fields: fields{
				name:  "test2",
				rate:  0,
				burst: 10,
			},
			errStr: "rate must be positive for group test2",
			name:   "0 rate",
		},
		{
			fields: fields{
				name:  "test3",
				rate:  5,
				burst: 0,
			},
			errStr: "burst must be positive for group test3",
			name:   "0 burst",
		},

		{
			fields: fields{
				name:  "test1",
				rate:  -1,
				burst: 10,
			},
			errStr: "rate must be positive for group test1",
			name:   "negative rate",
		},

		{
			fields: fields{
				name:  "test%$#",
				rate:  1,
				burst: 10,
			},
			errStr: "must match pattern",
			name:   "wrong name",
		},
		{
			fields: fields{
				name:  "tooo_long_group_name_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				rate:  1,
				burst: 10,
			},
			errStr: "group name must be less then 64 chars for group",
			name:   "too long name",
		},
	}

	for _, tt := range tests {

		gl := NewGroupList()
		err := gl.Add(tt.fields.name, tt.fields.rate, tt.fields.burst)
		if err != nil {
			require.ErrorIs(t, err, errors.ErrInvalidData)
			require.NotEmpty(t, tt.errStr, "Test %s expects an error, but got NIL", tt.name)
			assert.Contains(t, err.Error(), tt.errStr, "Test %s expects error %s, but got %s", tt.name, tt.errStr, err.Error())
		}
	}
}
