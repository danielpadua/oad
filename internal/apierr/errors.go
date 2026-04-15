// Package apierr defines structured API error types that map domain errors
// to HTTP responses. Handlers use these types to produce consistent error payloads.
package apierr

import "net/http"

// APIError represents a structured error returned by the API.
// HTTPStatus is excluded from JSON serialization; it drives the response code.
type APIError struct {
	HTTPStatus int      `json:"-"`
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Details    []string `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return e.Message
}

func NotFound(resource string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusNotFound,
		Code:       "NOT_FOUND",
		Message:    resource + " not found",
	}
}

func Conflict(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusConflict,
		Code:       "CONFLICT",
		Message:    message,
	}
}

func BadRequest(message string, details ...string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusBadRequest,
		Code:       "BAD_REQUEST",
		Message:    message,
		Details:    details,
	}
}

func ValidationFailed(details ...string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusBadRequest,
		Code:       "VALIDATION_FAILED",
		Message:    "request validation failed",
		Details:    details,
	}
}

func Unauthorized(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusUnauthorized,
		Code:       "UNAUTHORIZED",
		Message:    message,
	}
}

func Forbidden(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusForbidden,
		Code:       "FORBIDDEN",
		Message:    message,
	}
}

func UnprocessableEntity(message string, details ...string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusUnprocessableEntity,
		Code:       "UNPROCESSABLE_ENTITY",
		Message:    message,
		Details:    details,
	}
}

func Internal(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       "INTERNAL_ERROR",
		Message:    message,
	}
}

func ServiceUnavailable(message string) *APIError {
	return &APIError{
		HTTPStatus: http.StatusServiceUnavailable,
		Code:       "SERVICE_UNAVAILABLE",
		Message:    message,
	}
}
