package repository

import (
	"errors"
	"fmt"
)

// Common repository errors
var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrDatabase      = errors.New("database error")
)

// NotFoundError represents a resource not found error
type NotFoundError struct {
	Resource string
	ID       interface{}
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %v", e.Resource, e.ID)
}

func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// AlreadyExistsError represents a duplicate resource error
type AlreadyExistsError struct {
	Resource string
	Field    string
	Value    interface{}
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("%s already exists with %s: %v", e.Resource, e.Field, e.Value)
}

func (e *AlreadyExistsError) Is(target error) bool {
	return target == ErrAlreadyExists
}

// ValidationError represents invalid input error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}

// DatabaseError wraps database errors
type DatabaseError struct {
	Op  string
	Err error
}

func (e *DatabaseError) Error() string {
	return fmt.Sprintf("database error during %s: %v", e.Op, e.Err)
}

func (e *DatabaseError) Is(target error) bool {
	return target == ErrDatabase
}

func (e *DatabaseError) Unwrap() error {
	return e.Err
}
