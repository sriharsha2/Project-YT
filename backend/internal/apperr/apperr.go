// Package apperr defines the domain error type that services return and the HTTP layer maps to responses.
package apperr

import "errors"

// Kind classifies an error so the edge can choose a status code without inspecting messages.
type Kind int

// Error kinds. KindInternal is the zero value so an unclassified error is never mistaken for a client error.
const (
	KindInternal Kind = iota
	KindValidation
	KindNotFound
)

// Error is a classified error whose Message is safe to return to API clients.
// Cause holds internal detail for logs and errors.Is/As, and is never sent to clients.
type Error struct {
	Kind    Kind
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// Validation reports input that the client must fix before retrying.
func Validation(message string) *Error {
	return &Error{Kind: KindValidation, Message: message}
}

// NotFound reports that the requested resource does not exist.
func NotFound(message string) *Error {
	return &Error{Kind: KindNotFound, Message: message}
}

// As returns the *Error in err's chain, or false when err is unclassified.
func As(err error) (*Error, bool) {
	var appErr *Error
	ok := errors.As(err, &appErr)
	return appErr, ok
}
