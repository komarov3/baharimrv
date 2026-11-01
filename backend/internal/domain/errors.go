// Package domain carries the errors a client can act on. A service rejects a
// request with a reason the user sees; anything else stays an internal failure.
package domain

import (
	"fmt"
	"net/http"
)

type Kind struct {
	Status int
	Code   string
}

var (
	// Rejected: well-formed request, but the resource's state forbids it.
	KindRejected = Kind{http.StatusConflict, "operation_rejected"}
	// Invalid: the client supplied data the server cannot accept.
	KindInvalid = Kind{http.StatusUnprocessableEntity, "invalid_resource"}
	// Unavailable: a downstream adapter could not answer.
	KindUnavailable = Kind{http.StatusServiceUnavailable, "adapter_unavailable"}
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func Reject(message string, args ...any) *Error  { return newError(KindRejected, "", message, args...) }
func Invalid(message string, args ...any) *Error { return newError(KindInvalid, "", message, args...) }
func Unavailable(message string, args ...any) *Error {
	return newError(KindUnavailable, "", message, args...)
}

func newError(kind Kind, code, message string, args ...any) *Error {
	if code == "" {
		code = kind.Code
	}
	return &Error{Kind: kind, Code: code, Message: fmt.Sprintf(message, args...)}
}
