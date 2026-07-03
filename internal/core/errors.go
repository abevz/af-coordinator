package core

import "fmt"

// ErrorCode constants used across the API.
const (
	ErrValidationFailed = "validation_failed"
	ErrNotFound         = "not_found"
	ErrConflict         = "version_conflict"
	ErrLeaseHeld        = "lease_held"
	ErrLeaseExpired     = "lease_expired"
)

// APIError is the standard error envelope returned by the daemon.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// APIErrorResponse is the outer wrapper for an error response.
type APIErrorResponse struct {
	Error APIError `json:"error"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewAPIError creates an APIError.
func NewAPIError(code, msg string) APIError {
	return APIError{Code: code, Message: msg}
}
