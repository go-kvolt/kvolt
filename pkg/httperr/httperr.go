package httperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a structured API error. Handlers can return it; KVolt maps it to JSON.
//
//	return httperr.New(http.StatusBadRequest, "invalid_json", "Body could not be parsed")
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"error"`
	Message string `json:"message,omitempty"`
	Details any    `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Code
}

// New builds an HTTP error with a machine code and human message.
func New(status int, code, message string) *Error {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	if code == "" {
		code = "error"
	}
	return &Error{Status: status, Code: code, Message: message}
}

// WithDetails attaches optional structured details (validation fields, etc.).
func (e *Error) WithDetails(details any) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Details = details
	return &cp
}

// BadRequest is a 400 helper.
func BadRequest(code, message string) *Error {
	return New(http.StatusBadRequest, code, message)
}

// Unauthorized is a 401 helper.
func Unauthorized(code, message string) *Error {
	if code == "" {
		code = "unauthorized"
	}
	return New(http.StatusUnauthorized, code, message)
}

// Forbidden is a 403 helper.
func Forbidden(code, message string) *Error {
	if code == "" {
		code = "forbidden"
	}
	return New(http.StatusForbidden, code, message)
}

// NotFound is a 404 helper.
func NotFound(code, message string) *Error {
	if code == "" {
		code = "not_found"
	}
	return New(http.StatusNotFound, code, message)
}

// TooManyRequests is a 429 helper.
func TooManyRequests(message string) *Error {
	if message == "" {
		message = "rate limit exceeded"
	}
	return New(http.StatusTooManyRequests, "rate_limited", message)
}

// Conflict is a 409 helper.
func Conflict(code, message string) *Error {
	if code == "" {
		code = "conflict"
	}
	return New(http.StatusConflict, code, message)
}

// UnprocessableEntity is a 422 helper.
func UnprocessableEntity(code, message string) *Error {
	if code == "" {
		code = "unprocessable_entity"
	}
	return New(http.StatusUnprocessableEntity, code, message)
}

// PayloadTooLarge is a 413 helper.
func PayloadTooLarge(message string) *Error {
	if message == "" {
		message = "payload too large"
	}
	return New(http.StatusRequestEntityTooLarge, "payload_too_large", message)
}

// ServiceUnavailable is a 503 helper.
func ServiceUnavailable(message string) *Error {
	if message == "" {
		message = "service unavailable"
	}
	return New(http.StatusServiceUnavailable, "unavailable", message)
}

// Internal is a 500 helper (generic message for clients).
func Internal(message string) *Error {
	if message == "" {
		message = "internal server error"
	}
	return New(http.StatusInternalServerError, "internal_error", message)
}

// As extracts *Error from an error chain (supports wrapping via errors.As).
func As(err error) (*Error, bool) {
	if err == nil {
		return nil, false
	}
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
