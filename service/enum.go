package service

const (
	RESPONSE_OK                         string = "0"                 // no limits exceeded
	RESPONSE_LIMIT                      string = "1"                 // limit exceeded
	RESPONSE_ERROR_INTERNAL             string = "e:internal"        // internal service error
	RESPONSE_ERROR_TOO_MANY_CONNECTIONS string = "e:busy"            // too many open connections
	RESPONSE_ERROR_READ_TIMEOUT         string = "e:read_timeout"    // read timeout
	RESPONSE_ERROR_INVALID_DATA         string = "e:invalid_data"    // invalid request
	RESPONSE_ERROR_GROUP_NOT_FOUND      string = "e:group_not_found" // group not found
)
