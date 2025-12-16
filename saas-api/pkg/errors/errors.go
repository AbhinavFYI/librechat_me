package errors

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code    string
	Message string
	Status  int
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Predefined errors
var (
	ErrNotFound = &AppError{
		Code:    "NOT_FOUND",
		Message: "Resource not found",
		Status:  http.StatusNotFound,
	}

	ErrUnauthorized = &AppError{
		Code:    "UNAUTHORIZED",
		Message: "Unauthorized access",
		Status:  http.StatusUnauthorized,
	}

	ErrForbidden = &AppError{
		Code:    "FORBIDDEN",
		Message: "Forbidden access",
		Status:  http.StatusForbidden,
	}

	ErrBadRequest = &AppError{
		Code:    "BAD_REQUEST",
		Message: "Invalid request",
		Status:  http.StatusBadRequest,
	}

	ErrInternalServer = &AppError{
		Code:    "INTERNAL_ERROR",
		Message: "Internal server error",
		Status:  http.StatusInternalServerError,
	}

	ErrConflict = &AppError{
		Code:    "CONFLICT",
		Message: "Resource conflict",
		Status:  http.StatusConflict,
	}

	ErrValidation = &AppError{
		Code:    "VALIDATION_ERROR",
		Message: "Validation failed",
		Status:  http.StatusBadRequest,
	}
)

func NewError(code, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

func WrapError(err error, code, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
		Err:     err,
	}
}

// ErrorResponse is a common error response format
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}
