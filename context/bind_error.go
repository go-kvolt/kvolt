package context

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/go-playground/validator/v10"
)

// BindError marks an error caused by the content of the request: a body that is not
// valid JSON, a value of the wrong type, a parameter that does not parse, or a failed
// `validate` rule. It is the client's mistake, so a handler that returns it answers
// 400 (or 422 for validation) instead of 500.
//
// The original error is kept: Error() returns its text unchanged and errors.As /
// errors.Is reach it through Unwrap.
type BindError struct {
	Err error
	// Validation is true when the input parsed but failed a `validate` rule.
	Validation bool
}

func (e *BindError) Error() string {
	if e == nil || e.Err == nil {
		return "bind error"
	}
	return e.Err.Error()
}

func (e *BindError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// bindErr wraps a non-nil input error as a *BindError.
func bindErr(err error) error {
	if err == nil {
		return nil
	}
	return &BindError{Err: err}
}

// validationErr wraps a failed validation. A misuse of the validator (a target that is
// not a struct) is a programming error and stays a plain error.
func validationErr(err error) error {
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return err
	}
	return &BindError{Err: err, Validation: true}
}

// httpError maps the bind error to the response sent to the client. Messages name the
// offending field but do not echo Go type names or parser internals.
func (e *BindError) httpError() *httperr.Error {
	if e.Validation {
		out := httperr.UnprocessableEntity("validation_failed", "request validation failed")
		var fields validator.ValidationErrors
		if errors.As(e.Err, &fields) {
			details := make([]map[string]string, 0, len(fields))
			for _, f := range fields {
				details = append(details, map[string]string{"field": f.Field(), "rule": f.Tag()})
			}
			out = out.WithDetails(details)
		}
		return out
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(e.Err, &typeErr) && typeErr.Field != "" {
		return httperr.BadRequest("invalid_request", fmt.Sprintf("invalid value for field %q", typeErr.Field))
	}
	return httperr.BadRequest("invalid_request", "request could not be parsed")
}
