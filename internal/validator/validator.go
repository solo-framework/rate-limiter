package validator

import (
	"fmt"
	"ratelimiter/internal/errors"
	"regexp"
)

var pattern string = `^[a-zA-Z0-9_]+$`
var groupNameRegex regexp.Regexp = *regexp.MustCompile(pattern)

func ValidateGroup(name string, rateValue float64, burstValue int) error {

	if name == "" {
		return errors.NewError("group name must not be empty", errors.ErrInvalidData)
	}

	if !groupNameRegex.MatchString(name) {
		return errors.NewError(
			fmt.Sprintf("group name for '%s' must match pattern %s", name, pattern),
			errors.ErrInvalidData,
		)
	}

	if len(name) > 64 {
		return errors.NewError(
			fmt.Sprintf("group name must be less then 64 chars for group %s, got %v", name, len(name)),
			errors.ErrInvalidData,
		)
	}

	if rateValue <= 0 {
		return errors.NewError(
			fmt.Sprintf("rate must be positive for group %s, got %v", name, rateValue),
			errors.ErrInvalidData,
		)
	}

	if burstValue <= 0 {
		return errors.NewError(
			fmt.Sprintf("burst must be positive for group %s, got %v", name, burstValue),
			errors.ErrInvalidData,
		)
	}

	return nil
}
