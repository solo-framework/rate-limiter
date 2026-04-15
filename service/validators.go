package service

// import "fmt"

// func validateGroup(name string, rateValue float64, burstValue int) error {

// 	if name == "" {
// 		return NewError("group name must not be empty", ErrInvalidData)
// 	}

// 	if !groupNameRegex.MatchString(name) {
// 		return NewError(
// 			fmt.Sprintf("group name for '%s' must match pattern %s", name, pattern),
// 			ErrInvalidData,
// 		)
// 	}

// 	if len(name) > 64 {
// 		return NewError(
// 			fmt.Sprintf("group name must be less then 64 chars for group %s", name),
// 			ErrInvalidData,
// 		)
// 	}

// 	if rateValue <= 0 {
// 		return NewError(
// 			fmt.Sprintf("rate must be positive for group %s", name),
// 			ErrInvalidData,
// 		)
// 	}

// 	if burstValue <= 0 {
// 		return NewError(
// 			fmt.Sprintf("burst must be positive for group %s", name),
// 			ErrInvalidData,
// 		)
// 	}

// 	return nil
// }
