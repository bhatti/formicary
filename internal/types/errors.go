package types

import (
	"fmt"
	"reflect"
)

// BaseError represents a base struct for common errors
type BaseError struct {
	Message  interface{} `json:"message"`
	Internal error       `json:"-"` // Stores the error returned by an external dependency
}

// NotFoundError represents an error when an object is not found.
type NotFoundError struct {
	*BaseError
}

// ConflictError represents an error when an object has conflicts
type ConflictError struct {
	*BaseError
}

// FatalError represents a hard error
type FatalError struct {
	*BaseError
}

// PermissionError represents an error when an object cannot be accessed due to security policies.
type PermissionError struct {
	*BaseError
}

// QuotaExceededError represents an error when subscription quota is exceeded
type QuotaExceededError struct {
	*BaseError
}

// DuplicateError represents an error when an object already exists.
type DuplicateError struct {
	*BaseError
}

// JobRequeueError represents an error when a job needs to be requeue
type JobRequeueError struct {
	*BaseError
}

// ValidationError represents an error when an object is not valid.
type ValidationError struct {
	*BaseError
}

// NewBaseError creates a new FoundError instance.
func NewBaseError(message ...interface{}) *BaseError {
	e := &BaseError{}
	if len(message) > 0 {
		e.Message = message[0]
	}
	return e
}

// NewPermissionError creates a new PermissionError instance.
func NewPermissionError(message ...interface{}) *PermissionError {
	return &PermissionError{BaseError: NewBaseError(message...)}
}

// NewQuotaExceededError creates a new QuotaExceededError instance.
func NewQuotaExceededError(message ...interface{}) *QuotaExceededError {
	return &QuotaExceededError{BaseError: NewBaseError(message...)}
}

// NewValidationError creates a new ValidationError instance.
func NewValidationError(message ...interface{}) *ValidationError {
	return &ValidationError{BaseError: NewBaseError(message...)}
}

// NewDuplicateError creates a new DuplicateError instance.
func NewDuplicateError(message ...interface{}) *DuplicateError {
	return &DuplicateError{BaseError: NewBaseError(message...)}
}

// NewNotFoundError creates a new FoundError instance.
func NewNotFoundError(message ...interface{}) *NotFoundError {
	return &NotFoundError{BaseError: NewBaseError(message...)}
}

// NewJobRequeueError creates a new JobRequeueError instance.
func NewJobRequeueError(message ...interface{}) *JobRequeueError {
	return &JobRequeueError{BaseError: NewBaseError(message...)}
}

// NewFatalError creates a new FatalError instance.
func NewFatalError(message ...interface{}) *FatalError {
	return &FatalError{BaseError: NewBaseError(message...)}
}

// NewConflictError creates a new ConflictError instance.
func NewConflictError(message ...interface{}) *ConflictError {
	return &ConflictError{BaseError: NewBaseError(message...)}
}

// Error makes it compatible with `error` interface.
func (he *BaseError) Error() string {
	if he.Internal == nil {
		return fmt.Sprintf("%s: message=%v", reflect.TypeOf(he), he.Message)
	}
	return fmt.Sprintf("%s: message=%v, internal=%v", reflect.TypeOf(he), he.Message, he.Internal)
}

// SetInternal sets error to BaseError.Internal
func (he *BaseError) SetInternal(err error) *BaseError {
	he.Internal = err
	return he
}

// Unwrap satisfies the Go 1.13 error wrapper interface.
func (he *BaseError) Unwrap() error {
	return he.Internal
}
