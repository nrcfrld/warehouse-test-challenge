package model

import "errors"

// FieldError associates a validation error with an input.
type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// Error is a safe, public domain error.
type Error struct {
	Status  int
	Message string
	Fields  []FieldError
}

func (e *Error) Error() string { return e.Message }

// Invalid builds a client-visible field validation error.
func Invalid(field, reason string) error {
	return &Error{Status: 400, Message: "Validation failed", Fields: []FieldError{{field, reason}}}
}

// NotFound identifies a missing or soft-deleted resource.
func NotFound(resource string) error { return &Error{Status: 404, Message: resource + " not found"} }

// Conflict identifies a domain conflict.
func Conflict(field, message string) error {
	return &Error{Status: 409, Message: message, Fields: []FieldError{{field, message}}}
}

// Public extracts a safe error through wrapping.
func Public(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Status: 500, Message: "An unexpected error occurred"}
}
