package dto

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ErrorResponse is the standard error response format
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information including request tracing
type ErrorDetail struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	RequestID string                 `json:"request_id"`
}

// BadRequest returns a 400 Bad Request error
func BadRequest(c *gin.Context, message string, details map[string]interface{}) {
	c.JSON(400, ErrorResponse{
		Error: ErrorDetail{
			Code:      "bad_request",
			Message:   message,
			Details:   details,
			RequestID: c.GetString("request_id"),
		},
	})
}

// ValidationError returns a 400 error with field-level validation details
func ValidationError(c *gin.Context, err error) {
	details := make(map[string]interface{})

	// Extract field-level errors from validator
	if ve, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range ve {
			details[fe.Field()] = formatValidationError(fe)
		}
	} else {
		details["error"] = err.Error()
	}

	c.JSON(400, ErrorResponse{
		Error: ErrorDetail{
			Code:      "validation_error",
			Message:   "Invalid request parameters",
			Details:   details,
			RequestID: c.GetString("request_id"),
		},
	})
}

// NotFound returns a 404 Not Found error
func NotFound(c *gin.Context, resource string, id interface{}) {
	c.JSON(404, ErrorResponse{
		Error: ErrorDetail{
			Code:    "not_found",
			Message: resource + " not found",
			Details: map[string]interface{}{
				"resource": resource,
				"id":       id,
			},
			RequestID: c.GetString("request_id"),
		},
	})
}

// Conflict returns a 409 Conflict error
func Conflict(c *gin.Context, message string, details map[string]interface{}) {
	c.JSON(409, ErrorResponse{
		Error: ErrorDetail{
			Code:      "conflict",
			Message:   message,
			Details:   details,
			RequestID: c.GetString("request_id"),
		},
	})
}

// InternalError returns a 500 Internal Server Error
// Logs full error for debugging but returns safe message to client
func InternalError(c *gin.Context, err error) {
	// Log full error for debugging
	slog.Error("internal server error",
		"error", err,
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
		"request_id", c.GetString("request_id"),
	)

	// Return safe message to client
	c.JSON(500, ErrorResponse{
		Error: ErrorDetail{
			Code:      "internal_error",
			Message:   "Internal server error",
			RequestID: c.GetString("request_id"),
		},
	})
}

// formatValidationError formats a validator.FieldError into a human-readable message
func formatValidationError(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "field is required"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	case "eq":
		return fmt.Sprintf("must be equal to %s", fe.Param())
	default:
		return fe.Error()
	}
}
