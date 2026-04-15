package control

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// There is no structs validation for now because creating groups
// validates deeper in code. But i keep this in mind

type responseStruct struct {
	Error bool `json:"error"`
	Data  any  `json:"data"`
}

type addGroupRequest struct {
	Name  string    `json:"name"`
	Rate  JsonFloat `json:"rate"`
	Burst JsonInt   `json:"burst"`
}

type updateGroupRequest struct {
	Name  string    `json:"name"`
	Rate  JsonFloat `json:"rate"`
	Burst JsonInt   `json:"burst"`
}

type deleteGroupRequest struct {
	Name string `json:"name"`
}

func DecodeRequest[T any](body io.ReadCloser) (T, error) {
	defer body.Close()

	var req T
	dec := json.NewDecoder(body)
	if err := dec.Decode(&req); err != nil {
		return req, err
	}

	return req, nil
}

// JSON numbers can be in int or string format. We need parse it
type JsonFloat float64

func (s *JsonFloat) UnmarshalJSON(data []byte) error {
	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}

	switch value := val.(type) {
	case float64:
		*s = JsonFloat(value)
	case string:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}

		*s = JsonFloat(parsed)
	default:
		return fmt.Errorf("unknown type for JsonFloat: %T", val)
	}

	return nil
}

type JsonInt int

func (s *JsonInt) UnmarshalJSON(data []byte) error {
	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}

	switch value := val.(type) {
	case int:
	// case int16:
	// case int32:
	// case int64:
	case float64:
		// case float32:
		*s = JsonInt(value)
	case string:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return err
		}

		*s = JsonInt(parsed)
	default:
		return fmt.Errorf("unknown type for JsonFloat: %T", val)
	}

	return nil
}
